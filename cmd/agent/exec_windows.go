//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func executeCommand(command string) ([]byte, error) {
	// Force code page 65001 (UTF-8) before executing the command
	// We chain "chcp 65001" with the actual command
	cmd := exec.Command("cmd", "/C", "chcp 65001 > nul && "+command)

	// Hide the command window
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	// Capture both stdout and stderr
	return cmd.CombinedOutput()
}
