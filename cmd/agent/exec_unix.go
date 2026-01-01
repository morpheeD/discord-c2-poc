//go:build !windows

package main

import (
	"os/exec"
)

func executeCommand(command string) ([]byte, error) {
	cmd := exec.Command("sh", "-c", command)
	return cmd.CombinedOutput()
}

func installPersistence() (string, error) {
	return "Persistence not implemented for Unix", nil
}
