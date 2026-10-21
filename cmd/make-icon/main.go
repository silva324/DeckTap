package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	outDir := "build/icon.iconset"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	_ = os.MkdirAll(outDir, 0o755)

	sizes := []int{16, 32, 64, 128, 256, 512, 1024}
	for _, sz := range sizes {
		img := drawIcon(sz)
		var name string
		switch sz {
		case 16:
			name = "icon_16x16.png"
		case 32:
			name = "icon_16x16@2x.png"
			save(img, filepath.Join(outDir, name))
			name = "icon_32x32.png"
		case 64:
			name = "icon_32x32@2x.png"
		case 128:
			name = "icon_128x128.png"
		case 256:
			name = "icon_128x128@2x.png"
			save(img, filepath.Join(outDir, name))
			name = "icon_256x256.png"
		case 512:
			name = "icon_256x256@2x.png"
			save(img, filepath.Join(outDir, name))
			name = "icon_512x512.png"
		case 1024:
			name = "icon_512x512@2x.png"
		}
		save(img, filepath.Join(outDir, name))
	}
}

func save(img image.Image, path string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	_ = png.Encode(f, img)
}

func drawIcon(sz int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, sz, sz))
	s := float64(sz)
	darkBg := color.RGBA{15, 23, 42, 255}
	slideBg := color.RGBA{30, 41, 59, 255}
	cyanBorder := color.RGBA{56, 189, 248, 255}
	white := color.RGBA{248, 250, 252, 255}
	slateLine := color.RGBA{100, 116, 139, 255}
	emerald := color.RGBA{52, 211, 153, 255}
	greenBadge := color.RGBA{16, 185, 129, 255}
	badgeBg := color.RGBA{9, 13, 22, 255}

	corner := s * 0.22
	// Squircle background
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			fx, fy := float64(x), float64(y)
			if inRoundedRect(fx, fy, s*0.06, s*0.06, s*0.88, s*0.88, corner) {
				img.Set(x, y, darkBg)
			}
		}
	}

	// Slide Card (center)
	cardX := s * 0.26
	cardY := s * 0.28
	cardW := s * 0.48
	cardH := s * 0.32
	cardCorner := s * 0.04
	borderW := math.Max(1.0, s*0.012)
	for y := int(cardY); y < int(cardY+cardH); y++ {
		for x := int(cardX); x < int(cardX+cardW); x++ {
			fx, fy := float64(x), float64(y)
			if inRoundedRect(fx, fy, cardX, cardY, cardW, cardH, cardCorner) {
				if !inRoundedRect(fx, fy, cardX+borderW, cardY+borderW, cardW-borderW*2, cardH-borderW*2, cardCorner-borderW) {
					img.Set(x, y, cyanBorder)
				} else {
					img.Set(x, y, slideBg)
				}
			}
		}
	}

	// Slide Title Bar Line
	barX := cardX + (cardW * 0.12)
	barY := cardY + (cardH * 0.2)
	fillRect(img, barX, barY, cardW*0.48, cardH*0.09, white)

	// Slide Body Content Lines
	fillRect(img, barX, barY+(cardH*0.22), cardW*0.75, cardH*0.065, slateLine)
	fillRect(img, barX, barY+(cardH*0.38), cardW*0.55, cardH*0.065, slateLine)

	// Secure E2EE Check Badge (Top Right of slide card)
	badgeCX := cardX + cardW - (s * 0.02)
	badgeCY := cardY - (s * 0.02)
	badgeR := s * 0.065
	for y := int(badgeCY - badgeR); y <= int(badgeCY+badgeR); y++ {
		for x := int(badgeCX - badgeR); x <= int(badgeCX+badgeR); x++ {
			dx := float64(x) - badgeCX
			dy := float64(y) - badgeCY
			distSq := dx*dx + dy*dy
			if distSq <= badgeR*badgeR {
				if distSq >= (badgeR-borderW)*(badgeR-borderW) {
					img.Set(x, y, greenBadge)
				} else {
					img.Set(x, y, badgeBg)
				}
			}
		}
	}
	// Checkmark inside badge
	checkW := math.Max(1.0, s*0.015)
	fillRect(img, badgeCX-badgeR*0.3, badgeCY, badgeR*0.35, checkW, greenBadge)
	fillRect(img, badgeCX, badgeCY-badgeR*0.35, checkW, badgeR*0.7, greenBadge)

	// Dynamic Swipe Arc (Emerald Gesture Indicator)
	arcCY := s * 0.72
	arcCX := s * 0.5
	arcRX := s * 0.32
	arcRY := s * 0.12
	arcThick := math.Max(2.0, s*0.025)
	for y := int(arcCY); y < int(arcCY+arcRY+arcThick*2); y++ {
		for x := int(arcCX - arcRX - arcThick); x <= int(arcCX+arcRX+arcThick); x++ {
			fx := float64(x) - arcCX
			fy := float64(y) - arcCY
			normDist := (fx*fx)/(arcRX*arcRX) + (fy*fy)/(arcRY*arcRY)
			if normDist >= 0.85 && normDist <= 1.25 && fy >= 0 {
				img.Set(x, y, emerald)
			}
		}
	}

	return img
}

func fillRect(img *image.RGBA, x, y, w, h float64, c color.Color) {
	for j := int(y); j < int(y+h); j++ {
		for i := int(x); i < int(x+w); i++ {
			img.Set(i, j, c)
		}
	}
}

func inRoundedRect(x, y, rx, ry, rw, rh, r float64) bool {
	if x < rx || x > rx+rw || y < ry || y > ry+rh {
		return false
	}
	dx := math.Max(math.Max(rx+r-x, 0), x-(rx+rw-r))
	dy := math.Max(math.Max(ry+r-y, 0), y-(ry+rh-r))
	return (dx*dx + dy*dy) <= (r * r)
}
