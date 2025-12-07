//go:build !windows

package updater

import "os/exec"

// hideWindow is a no-op on non-Windows platforms
func hideWindow(cmd *exec.Cmd) {
	// No-op on non-Windows
}
