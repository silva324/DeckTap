//go:build notray

package tray

import "github.com/silva324/DeckTap/internal/app"

func Supported() bool                                      { return false }
func Quit()                                                {}
func Run(h *app.Host, openURL func(string), onExit func()) { onExit() }

var _ = iconTemplate
var _ = iconICO
var _ = iconPNG
