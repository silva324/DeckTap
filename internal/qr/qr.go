// Package qr is a small, dependency-free QR Code encoder (byte mode, ECC level M).
//
// It covers versions 1–40, which is far more than the ~150-byte pairing URL needs.
// The output is validated in tests by decoding it with an independent decoder.
package qr

import (
	"errors"
	"fmt"
	"strings"
)

// Code is an encoded QR symbol. Modules[y][x] == true means a dark module.
type Code struct {
	Version int
	Size    int
	Mask    int
	Modules [][]bool
}

// ErrTooLong is returned when the payload does not fit in version 40-M.
var ErrTooLong = errors.New("qr: data too long")

// Encode encodes data in byte mode with error correction level M.
func Encode(data []byte) (*Code, error) {
	version := 0
	for v := 1; v <= 40; v++ {
		if 4+charCountBits(v)+8*len(data) <= dataCodewords(v)*8 {
			version = v
			break
		}
	}
	if version == 0 {
		return nil, ErrTooLong
	}
	codewords := addECC(buildData(data, version), version)

	c := newCanvas(version)
	c.drawFunctionPatterns()
	c.drawCodewords(codewords)

	best, bestPenalty := 0, -1
	for m := 0; m < 8; m++ {
		c.applyMask(m)
		c.drawFormatBits(m)
		if p := c.penalty(); bestPenalty < 0 || p < bestPenalty {
			best, bestPenalty = m, p
		}
		c.applyMask(m) // XOR again = undo
	}
	c.applyMask(best)
	c.drawFormatBits(best)
	return &Code{Version: version, Size: c.size, Mask: best, Modules: c.mod}, nil
}

// ---------------------------------------------------------------- data

func charCountBits(v int) int {
	if v <= 9 {
		return 8
	}
	return 16
}

func dataCodewords(v int) int {
	t := rsBlocksM[v]
	n := 0
	for i := 0; i < len(t); i += 3 {
		n += t[i] * t[i+2]
	}
	return n
}

type bitBuf struct {
	b []byte
	n int
}

func (bb *bitBuf) put(val, bits int) {
	for i := bits - 1; i >= 0; i-- {
		if bb.n%8 == 0 {
			bb.b = append(bb.b, 0)
		}
		if (val>>uint(i))&1 == 1 {
			bb.b[bb.n/8] |= 0x80 >> uint(bb.n%8)
		}
		bb.n++
	}
}

func buildData(data []byte, v int) []byte {
	capBits := dataCodewords(v) * 8
	bb := &bitBuf{}
	bb.put(0x4, 4) // byte mode
	bb.put(len(data), charCountBits(v))
	for _, d := range data {
		bb.put(int(d), 8)
	}
	term := capBits - bb.n
	if term > 4 {
		term = 4
	}
	bb.put(0, term)
	if bb.n%8 != 0 {
		bb.put(0, 8-bb.n%8)
	}
	for pad := 0; len(bb.b) < dataCodewords(v); pad++ {
		if pad%2 == 0 {
			bb.b = append(bb.b, 0xEC)
		} else {
			bb.b = append(bb.b, 0x11)
		}
	}
	return bb.b
}

func addECC(data []byte, v int) []byte {
	t := rsBlocksM[v]
	var dataBlocks, eccBlocks [][]byte
	off := 0
	for i := 0; i < len(t); i += 3 {
		count, total, dlen := t[i], t[i+1], t[i+2]
		for k := 0; k < count; k++ {
			blk := data[off : off+dlen]
			off += dlen
			dataBlocks = append(dataBlocks, blk)
			eccBlocks = append(eccBlocks, rsEncode(blk, total-dlen))
		}
	}
	var out []byte
	for _, blocks := range [][][]byte{dataBlocks, eccBlocks} {
		maxLen := 0
		for _, b := range blocks {
			if len(b) > maxLen {
				maxLen = len(b)
			}
		}
		for i := 0; i < maxLen; i++ {
			for _, b := range blocks {
				if i < len(b) {
					out = append(out, b[i])
				}
			}
		}
	}
	return out
}

// ---------------------------------------------------------------- Reed-Solomon over GF(256), poly 0x11D

var gfExp [512]byte
var gfLog [256]int

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		gfExp[i] = byte(x)
		gfLog[x] = i
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11D
		}
	}
	for i := 255; i < 512; i++ {
		gfExp[i] = gfExp[i-255]
	}
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[gfLog[a]+gfLog[b]]
}

func rsEncode(data []byte, degree int) []byte {
	gen := []byte{1}
	for i := 0; i < degree; i++ {
		next := make([]byte, len(gen)+1)
		for j, c := range gen {
			next[j] ^= c
			next[j+1] ^= gfMul(c, gfExp[i])
		}
		gen = next
	}
	res := make([]byte, len(data)+degree)
	copy(res, data)
	for i := range data {
		c := res[i]
		if c == 0 {
			continue
		}
		for j := 1; j < len(gen); j++ {
			res[i+j] ^= gfMul(gen[j], c)
		}
	}
	return res[len(data):]
}

// ---------------------------------------------------------------- matrix

type canvas struct {
	version int
	size    int
	mod     [][]bool
	fn      [][]bool
}

func newCanvas(v int) *canvas {
	size := 17 + 4*v
	c := &canvas{version: v, size: size, mod: make([][]bool, size), fn: make([][]bool, size)}
	for i := range c.mod {
		c.mod[i] = make([]bool, size)
		c.fn[i] = make([]bool, size)
	}
	return c
}

func (c *canvas) setFn(x, y int, dark bool) {
	c.mod[y][x] = dark
	c.fn[y][x] = true
}

func (c *canvas) drawFunctionPatterns() {
	s := c.size
	for i := 0; i < s; i++ {
		c.setFn(6, i, i%2 == 0)
		c.setFn(i, 6, i%2 == 0)
	}
	c.drawFinder(3, 3)
	c.drawFinder(s-4, 3)
	c.drawFinder(3, s-4)
	pos := alignPos[c.version]
	n := len(pos)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if (i == 0 && j == 0) || (i == 0 && j == n-1) || (i == n-1 && j == 0) {
				continue
			}
			c.drawAlignment(pos[i], pos[j])
		}
	}
	c.drawFormatBits(0) // reserve the area; real bits drawn after masking
	c.drawVersion()
}

func (c *canvas) drawFinder(cx, cy int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			x, y := cx+dx, cy+dy
			if x < 0 || y < 0 || x >= c.size || y >= c.size {
				continue
			}
			d := max(abs(dx), abs(dy))
			c.setFn(x, y, d != 2 && d != 4)
		}
	}
}

func (c *canvas) drawAlignment(cx, cy int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			c.setFn(cx+dx, cy+dy, max(abs(dx), abs(dy)) != 1)
		}
	}
}

func (c *canvas) drawFormatBits(mask int) {
	const eclM = 0 // format bits: L=1, M=0, Q=3, H=2
	data := eclM<<3 | mask
	rem := data
	for i := 0; i < 10; i++ {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}
	bits := (data<<10 | rem) ^ 0x5412
	bit := func(i int) bool { return (bits>>uint(i))&1 == 1 }
	s := c.size
	for i := 0; i <= 5; i++ {
		c.setFn(8, i, bit(i))
	}
	c.setFn(8, 7, bit(6))
	c.setFn(8, 8, bit(7))
	c.setFn(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		c.setFn(14-i, 8, bit(i))
	}
	for i := 0; i < 8; i++ {
		c.setFn(s-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		c.setFn(8, s-15+i, bit(i))
	}
	c.setFn(8, s-8, true) // dark module
}

func (c *canvas) drawVersion() {
	if c.version < 7 {
		return
	}
	rem := c.version
	for i := 0; i < 12; i++ {
		rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
	}
	bits := c.version<<12 | rem
	for i := 0; i < 18; i++ {
		dark := (bits>>uint(i))&1 == 1
		a, b := c.size-11+i%3, i/3
		c.setFn(a, b, dark)
		c.setFn(b, a, dark)
	}
}

func (c *canvas) drawCodewords(data []byte) {
	i := 0
	s := c.size
	for right := s - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < s; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				upward := ((right + 1) & 2) == 0
				y := vert
				if upward {
					y = s - 1 - vert
				}
				if !c.fn[y][x] && i < len(data)*8 {
					c.mod[y][x] = (data[i>>3]>>uint(7-(i&7)))&1 == 1
					i++
				}
			}
		}
	}
}

func (c *canvas) applyMask(m int) {
	for y := 0; y < c.size; y++ {
		for x := 0; x < c.size; x++ {
			if c.fn[y][x] {
				continue
			}
			var inv bool
			switch m {
			case 0:
				inv = (x+y)%2 == 0
			case 1:
				inv = y%2 == 0
			case 2:
				inv = x%3 == 0
			case 3:
				inv = (x+y)%3 == 0
			case 4:
				inv = (x/3+y/2)%2 == 0
			case 5:
				inv = x*y%2+x*y%3 == 0
			case 6:
				inv = (x*y%2+x*y%3)%2 == 0
			case 7:
				inv = ((x+y)%2+x*y%3)%2 == 0
			}
			if inv {
				c.mod[y][x] = !c.mod[y][x]
			}
		}
	}
}

// penalty implements the four ISO/IEC 18004 penalty rules.
func (c *canvas) penalty() int {
	s, p := c.size, 0
	get := func(x, y int, row bool) bool {
		if row {
			return c.mod[y][x]
		}
		return c.mod[x][y]
	}
	for _, row := range []bool{true, false} {
		for y := 0; y < s; y++ {
			run, prev := 0, false
			for x := 0; x < s; x++ {
				v := get(x, y, row)
				if x > 0 && v == prev {
					run++
				} else {
					if run >= 5 {
						p += 3 + run - 5
					}
					run, prev = 1, v
				}
				// rule 3: 1011101 with 4 light modules on one side
				if x >= 10 {
					pat := []bool{true, false, true, true, true, false, true}
					m1, m2 := true, true
					for k := 0; k < 7; k++ {
						if get(x-10+k, y, row) != pat[k] {
							m1 = false
						}
						if get(x-6+k, y, row) != pat[k] {
							m2 = false
						}
					}
					for k := 0; k < 4; k++ {
						if get(x-3+k, y, row) {
							m1 = false
						}
						if get(x-10+k, y, row) {
							m2 = false
						}
					}
					if m1 || m2 {
						p += 40
					}
				}
			}
			if run >= 5 {
				p += 3 + run - 5
			}
		}
	}
	dark := 0
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			if c.mod[y][x] {
				dark++
			}
			if x < s-1 && y < s-1 {
				v := c.mod[y][x]
				if v == c.mod[y][x+1] && v == c.mod[y+1][x] && v == c.mod[y+1][x+1] {
					p += 3
				}
			}
		}
	}
	total := s * s
	k := (abs(dark*20-total*10)+total-1)/total - 1
	if k > 0 {
		p += k * 10
	}
	return p
}

// ---------------------------------------------------------------- rendering

// SVG renders the code with a 4-module quiet zone.
func (q *Code) SVG(moduleSize int) string {
	const quiet = 4
	n := q.Size + 2*quiet
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" shape-rendering="crispEdges">`,
		n, n, n*moduleSize, n*moduleSize)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n, n)
	for y := 0; y < q.Size; y++ {
		for x := 0; x < q.Size; x++ {
			if q.Modules[y][x] {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}

// Terminal renders the code with half-block characters (2 rows per line).
func (q *Code) Terminal() string {
	const quiet = 2
	n := q.Size + 2*quiet
	dark := func(x, y int) bool {
		x, y = x-quiet, y-quiet
		return x >= 0 && y >= 0 && x < q.Size && y < q.Size && q.Modules[y][x]
	}
	var b strings.Builder
	for y := 0; y < n; y += 2 {
		for x := 0; x < n; x++ {
			top, bot := !dark(x, y), !dark(x, y+1) // print light modules (dark-background terminals)
			if y+1 >= n {
				bot = false
			}
			switch {
			case top && bot:
				b.WriteRune('█')
			case top:
				b.WriteRune('▀')
			case bot:
				b.WriteRune('▄')
			default:
				b.WriteRune(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
