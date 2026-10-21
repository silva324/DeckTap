// Package tray shows DeckTap in the macOS menu bar / Windows notification area.
package tray

import _ "embed"

//go:embed icons/template.png
var iconTemplate []byte // macOS template image (black + alpha, adapts to light/dark menu bar)

//go:embed icons/icon.ico
var iconICO []byte // Windows needs .ico

//go:embed icons/icon.png
var iconPNG []byte
