//go:build windows

package updater

import (
	"os/exec"
	"syscall"
)

// hideWindow sets the SysProcAttr to hide the console window on Windows
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
