// Package nativewin provides native, in-app desktop windows for pairing and status
// without requiring the user to open an external web browser.
package nativewin

// WindowConfig contains parameters for presenting a native in-app window.
type WindowConfig struct {
	URL    string
	Title  string
	Width  int
	Height int
}

// Open opens or brings to the front a native in-app window with the given config.
func Open(cfg WindowConfig) {
	if cfg.Width <= 0 {
		cfg.Width = 480
	}
	if cfg.Height <= 0 {
		cfg.Height = 680
	}
	if cfg.Title == "" {
		cfg.Title = "DeckTap"
	}
	openPlatform(cfg)
}
