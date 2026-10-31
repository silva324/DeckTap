// Command make-icon draws every DeckTap icon from one definition, so the app icon,
// tray icons and web icon stay in sync (see internal/webui/static/icon.svg).
//
//	go run ./cmd/make-icon [iconset-dir]        macOS .iconset (default build/icon.iconset)
//	go run ./cmd/make-icon -tray internal/tray/icons
//	                                            icon.png, icon.ico, template.png
//
// Shapes are signed distance fields with analytic anti-aliasing, in a 512-unit space.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// Volaro palette: horizon ground, ink, ink-muted and Dawn (DeckTap's accent).
var (
	horizon = color.NRGBA{0x0b, 0x14, 0x22, 0xff}
	ink     = color.NRGBA{0xf2, 0xf5, 0xfa, 0xff}
	muted   = color.NRGBA{0xa9, 0xb8, 0xcc, 0xff}
	dawn    = color.NRGBA{0xff, 0x9b, 0x80, 0xff}
	black   = color.NRGBA{0, 0, 0, 0xff}
)

func main() {
	tray := flag.String("tray", "", "write tray icons (icon.png, icon.ico, template.png) to this directory")
	flag.Parse()
	if *tray != "" {
		writeTray(*tray)
		return
	}
	dir := "build/icon.iconset"
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	writeIconset(dir)
}

func writeIconset(dir string) {
	must(os.MkdirAll(dir, 0o755))
	for _, e := range []struct {
		px   int
		name string
	}{
		{16, "icon_16x16.png"}, {32, "icon_16x16@2x.png"}, {32, "icon_32x32.png"}, {64, "icon_32x32@2x.png"},
		{128, "icon_128x128.png"}, {256, "icon_128x128@2x.png"}, {256, "icon_256x256.png"},
		{512, "icon_256x256@2x.png"}, {512, "icon_512x512.png"}, {1024, "icon_512x512@2x.png"},
	} {
		// macOS icon grid: the tile is 824/1024 of the canvas, centred.
		save(filepath.Join(dir, e.name), appIcon(e.px, 100.0/1024))
	}
}

func writeTray(dir string) {
	must(os.MkdirAll(dir, 0o755))
	save(filepath.Join(dir, "icon.png"), appIcon(64, 0))
	save(filepath.Join(dir, "template.png"), templateIcon(44))
	var pngs [][]byte
	sizes := []int{16, 24, 32, 48, 256}
	for _, px := range sizes {
		pngs = append(pngs, encode(appIcon(px, 0)))
	}
	must(os.WriteFile(filepath.Join(dir, "icon.ico"), ico(sizes, pngs), 0o644))
}

// appIcon is the full-colour mark: a slide with a Dawn tap on a horizon tile.
// inset is the transparent margin on each side, as a fraction of the canvas.
func appIcon(px int, inset float64) *image.NRGBA {
	small := px <= 32 // drop detail and thicken strokes so the mark survives tiny sizes
	return render(px, inset, func(p vec, c *canvas) {
		c.fill(sdRoundRect(p, vec{256, 256}, vec{256, 256}, 112), horizon)
		slideStroke, dotR := 18.0, 32.0
		if small {
			slideStroke, dotR = 30, 44
		}
		dot := vec{326, 300}
		if !small {
			c.fillAlpha(math.Abs(sdCircle(p, dot, 66))-5, dawn, 0.45) // ripple, beneath the slide
		}
		c.fill(math.Abs(sdRoundRect(p, vec{256, 236}, vec{132, 88}, 22))-slideStroke/2, ink)
		if !small {
			c.fill(sdCapsule(p, vec{172, 210}, vec{276, 210}, 7), muted)
			c.fill(sdCapsule(p, vec{172, 250}, vec{236, 250}, 7), muted)
		}
		c.fill(sdCircle(p, dot, dotR+16), horizon) // gap so the tap reads crisply
		c.fill(sdCircle(p, dot, dotR), dawn)
	})
}

// templateIcon is the macOS menu-bar glyph: black plus alpha, tinted by the system.
// The tap dot is separated from the slide by a clear gap so it reads at 22pt.
func templateIcon(px int) *image.NRGBA {
	return render(px, 0, func(p vec, c *canvas) {
		c.fill(math.Abs(sdRoundRect(p, vec{232, 236}, vec{196, 140}, 44))-22, black)
		dot := vec{392, 372}
		c.clear(sdCircle(p, dot, 118))
		c.fill(sdCircle(p, dot, 80), black)
	})
}

// ---------------------------------------------------------------- rendering

type vec struct{ x, y float64 }

func (a vec) sub(b vec) vec       { return vec{a.x - b.x, a.y - b.y} }
func (a vec) len() float64        { return math.Hypot(a.x, a.y) }
func (a vec) dot(b vec) float64   { return a.x*b.x + a.y*b.y }
func (a vec) scale(k float64) vec { return vec{a.x * k, a.y * k} }

func sdCircle(p, c vec, r float64) float64 { return p.sub(c).len() - r }

func sdRoundRect(p, c, half vec, r float64) float64 {
	q := vec{math.Abs(p.x-c.x) - (half.x - r), math.Abs(p.y-c.y) - (half.y - r)}
	outside := vec{math.Max(q.x, 0), math.Max(q.y, 0)}.len()
	return outside + math.Min(math.Max(q.x, q.y), 0) - r
}

func sdCapsule(p, a, b vec, r float64) float64 {
	pa, ba := p.sub(a), b.sub(a)
	h := math.Max(0, math.Min(1, pa.dot(ba)/ba.dot(ba)))
	return pa.sub(ba.scale(h)).len() - r
}

// canvas accumulates one pixel; distances are in 512-unit space.
type canvas struct {
	r, g, b, a float64
	unit       float64 // pixels per unit, for anti-aliasing
}

func (c *canvas) coverage(d float64) float64 { return math.Max(0, math.Min(1, 0.5-d*c.unit)) }

func (c *canvas) fill(d float64, col color.NRGBA) { c.fillAlpha(d, col, 1) }

func (c *canvas) fillAlpha(d float64, col color.NRGBA, opacity float64) {
	a := c.coverage(d) * opacity * float64(col.A) / 255
	if a == 0 {
		return
	}
	// Source-over with straight alpha.
	outA := a + c.a*(1-a)
	c.r = (float64(col.R)*a + c.r*c.a*(1-a)) / outA
	c.g = (float64(col.G)*a + c.g*c.a*(1-a)) / outA
	c.b = (float64(col.B)*a + c.b*c.a*(1-a)) / outA
	c.a = outA
}

// clear erases what is already drawn inside d < 0 (destination-out).
func (c *canvas) clear(d float64) { c.a *= 1 - c.coverage(d) }

func render(px int, inset float64, draw func(vec, *canvas)) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, px, px))
	margin := inset * float64(px)
	tile := float64(px) - 2*margin
	k := 512 / tile
	for y := 0; y < px; y++ {
		for x := 0; x < px; x++ {
			p := vec{(float64(x) + 0.5 - margin) * k, (float64(y) + 0.5 - margin) * k}
			c := canvas{unit: 1 / k}
			draw(p, &c)
			if c.a > 0 {
				img.SetNRGBA(x, y, color.NRGBA{u8(c.r), u8(c.g), u8(c.b), u8(c.a * 255)})
			}
		}
	}
	return img
}

func u8(v float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(255, v)))) }

// ---------------------------------------------------------------- output

// ico packs PNG images into a Windows .ico (PNG entries are supported since Vista).
func ico(sizes []int, pngs [][]byte) []byte {
	var buf bytes.Buffer
	n := len(pngs)
	_ = binary.Write(&buf, binary.LittleEndian, [3]uint16{0, 1, uint16(n)})
	offset := uint32(6 + 16*n)
	for i, data := range pngs {
		dim := uint8(sizes[i] % 256) // 0 means 256
		_ = binary.Write(&buf, binary.LittleEndian, struct {
			W, H, Colors, Reserved uint8
			Planes, BitCount       uint16
			Size, Offset           uint32
		}{dim, dim, 0, 0, 1, 32, uint32(len(data)), offset})
		offset += uint32(len(data))
	}
	for _, data := range pngs {
		buf.Write(data)
	}
	return buf.Bytes()
}

func encode(img image.Image) []byte {
	var buf bytes.Buffer
	must(png.Encode(&buf, img))
	return buf.Bytes()
}

func save(path string, img image.Image) { must(os.WriteFile(path, encode(img), 0o644)) }

func must(err error) {
	if err != nil {
		panic(err)
	}
}
