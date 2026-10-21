//go:build notray

package tray

import "pptx-remote/internal/app"

func Supported() bool                                      { return false }
func Quit()                                                {}
func Run(h *app.Host, openURL func(string), onExit func()) { onExit() }

var _ = iconTemplate
var _ = iconICO
var _ = iconPNG
