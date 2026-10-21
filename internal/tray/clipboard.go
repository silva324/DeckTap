package tray

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
)

// CopyToClipboard copies a text string to the system clipboard.
func CopyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("cmd", "/c", "clip")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard")
	}

	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start clipboard command: %w", err)
	}

	if _, err := io.WriteString(in, text); err != nil {
		return err
	}
	_ = in.Close()

	return cmd.Wait()
}
