//go:build (!darwin && !windows) || (darwin && !cgo)

package nativewin

import (
	"os/exec"
	"runtime"
)

func openPlatform(cfg WindowConfig) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", cfg.URL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", cfg.URL)
	default:
		cmd = exec.Command("xdg-open", cfg.URL)
	}
	_ = cmd.Start()
}

// Supported reports false on platforms without native in-app window support.
func Supported() bool {
	return false
}
