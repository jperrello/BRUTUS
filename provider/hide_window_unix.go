//go:build !windows

package provider

import "os/exec"

func hideCommandWindow(cmd *exec.Cmd) {
	// No-op on non-Windows platforms
}
