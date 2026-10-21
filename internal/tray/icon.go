package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// GenerateTrayIcon creates a 32x32 template icon representing a presentation screen & remote signal.
func GenerateTrayIcon() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	// Draw outer presentation screen frame: x: 4..27, y: 5..21
	for x := 4; x <= 27; x++ {
		img.Set(x, 5, white)
		img.Set(x, 6, white)
		img.Set(x, 20, white)
		img.Set(x, 21, white)
	}
	for y := 5; y <= 21; y++ {
		img.Set(4, y, white)
		img.Set(5, y, white)
		img.Set(26, y, white)
		img.Set(27, y, white)
	}

	// Draw presenter stand: pole from y: 22..26 at center (x: 15..16)
	for y := 22; y <= 26; y++ {
		img.Set(15, y, white)
		img.Set(16, y, white)
	}
	// Base stand: x: 10..21 at y: 27..28
	for x := 10; x <= 21; x++ {
		img.Set(x, 27, white)
		img.Set(x, 28, white)
	}

	// Play arrow / presentation symbol inside screen (centered)
	// Arrow pointing right: x: 13..18, y: 10..16
	for y := 10; y <= 16; y++ {
		depth := (y - 10)
		if depth > 3 {
			depth = 6 - depth
		}
		for x := 13; x <= 13+depth+1; x++ {
			img.Set(x, y, white)
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
