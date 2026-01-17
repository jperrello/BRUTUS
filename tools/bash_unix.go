//go:build !windows

package tools

import "os/exec"

func hideCommandWindow(cmd *exec.Cmd) {
	// No-op on non-Windows platforms
}
