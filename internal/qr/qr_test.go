package qr

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestKnownVectorVersion1(t *testing.T) {
	// RS check against a published example (ISO 18004 Annex I, "01234567" is numeric;
	// we instead check the generator on a well-known byte vector).
	ecc := rsEncode([]byte{0x40, 0xd2, 0x75, 0x47, 0x76, 0x17, 0x32, 0x06, 0x27, 0x26, 0x96, 0xc6, 0xc6, 0x96, 0x70, 0xec}, 10)
	want := []byte{0xbc, 0x2a, 0x90, 0x13, 0x6b, 0xaf, 0xef, 0xfd, 0x4b, 0xe0}
	for i := range want {
		if ecc[i] != want[i] {
			t.Fatalf("rs mismatch at %d: got %x want %x", i, ecc, want)
		}
	}
}

func TestVersionSelection(t *testing.T) {
	cases := map[int]int{1: 1, 14: 1, 15: 2, 140: 8, 200: 10}
	for n, v := range cases {
		c, err := Encode([]byte(strings.Repeat("a", n)))
		if err != nil || c.Version != v {
			t.Fatalf("len %d: version %v err %v, want %d", n, c.Version, err, v)
		}
	}
	if _, err := Encode(make([]byte, 3000)); err != ErrTooLong {
		t.Fatal("expected ErrTooLong")
	}
}

// TestDump writes PNGs for external decoding when QR_DUMP_DIR is set (see scripts/verify_qr.py).
func TestDump(t *testing.T) {
	dir := os.Getenv("QR_DUMP_DIR")
	if dir == "" {
		t.Skip("QR_DUMP_DIR not set")
	}
	payloads := []string{
		"hello",
		"https://swift-fox-89.trycloudflare.com/#dGhpcyBpcyBhIHRva2Vu.Y2xvdWRmbGFyZS1jYW5ub3QtcmVhZC10aGlzLWtleQ",
		"https://gentle-orange-mountains-anchor.trycloudflare.com/#AAAAAAAAAAAAAAAAAAAAAA.BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		strings.Repeat("DeckTap-", 40),
	}
	for i, p := range payloads {
		c, err := Encode([]byte(p))
		if err != nil {
			t.Fatal(err)
		}
		writePNG(t, filepath.Join(dir, strconv.Itoa(i)+".png"), c)
		os.WriteFile(filepath.Join(dir, strconv.Itoa(i)+".txt"), []byte(p), 0o644)
	}
}

func writePNG(t *testing.T, path string, c *Code) {
	const scale, quiet = 8, 4
	n := (c.Size + 2*quiet) * scale
	img := image.NewGray(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			mx, my := x/scale-quiet, y/scale-quiet
			v := uint8(255)
			if mx >= 0 && my >= 0 && mx < c.Size && my < c.Size && c.Modules[my][mx] {
				v = 0
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, img)
}
