//go:build windows

package procutil

import (
	"os/exec"
	"syscall"
)

// Hide prevents a console window from flashing up for the child process.
func Hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
