//go:build windows

package nativewin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func openPlatform(cfg WindowConfig) {
	// 1. Try Microsoft Edge in standalone App Mode (pre-installed on Windows 10/11)
	edgePaths := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	for _, p := range edgePaths {
		if _, err := os.Stat(p); err == nil {
			cmd := exec.Command(p, fmt.Sprintf("--app=%s", cfg.URL), fmt.Sprintf("--window-size=%d,%d", cfg.Width, cfg.Height))
			if err := cmd.Start(); err == nil {
				return
			}
		}
	}

	// 2. Try Google Chrome in standalone App Mode
	chromePaths := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, p := range chromePaths {
		if _, err := os.Stat(p); err == nil {
			cmd := exec.Command(p, fmt.Sprintf("--app=%s", cfg.URL), fmt.Sprintf("--window-size=%d,%d", cfg.Width, cfg.Height))
			if err := cmd.Start(); err == nil {
				return
			}
		}
	}

	// 3. Fallback to default system shell handler
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", cfg.URL)
	_ = cmd.Start()
}

// Supported reports true on Windows.
func Supported() bool {
	return true
}
