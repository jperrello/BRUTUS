//go:build !windows

package provider

import "os/exec"

func hideWindow(cmd *exec.Cmd) {
	// No-op on Unix systems - no console windows to hide
}
