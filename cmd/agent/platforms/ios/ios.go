//go:build ios

package ios

import "fmt"

// IOSPlatform represents the iOS platform.
type IOSPlatform struct{}

// NewPlatform returns a new instance of the IOSPlatform.
func NewPlatform() *IOSPlatform {
	return &IOSPlatform{}
}

// ExecuteCommand executes a command on iOS.
func (p *IOSPlatform) ExecuteCommand(command string) ([]byte, error) {
	return nil, fmt.Errorf("command execution not implemented for iOS")
}

// InstallPersistence does nothing on iOS.
func (p *IOSPlatform) InstallPersistence() (string, error) {
	return "Persistence not implemented for iOS.", nil
}

// Init does nothing on iOS.
func (p *IOSPlatform) Init() error {
	return nil
}

// StartKeylogger does nothing on iOS.
func (p *IOSPlatform) StartKeylogger() {}

// GetKeylogs does nothing on iOS.
func (p *IOSPlatform) GetKeylogs() string {
	return "Keylogger not implemented for iOS."
}

// DumpBrowsers does nothing on iOS.
func (p *IOSPlatform) DumpBrowsers() string {
	return "Browser password dumping not implemented for iOS."
}

// Screenshot does nothing on iOS.
func (p *IOSPlatform) Screenshot() ([]byte, error) {
	return nil, fmt.Errorf("screenshot not implemented for iOS")
}

// RecordMicrophone does nothing on iOS.
func (p *IOSPlatform) RecordMicrophone() ([]byte, error) {
	return nil, fmt.Errorf("microphone recording not implemented for iOS")
}

// GetLocation does nothing on iOS.
func (p *IOSPlatform) GetLocation() ([]byte, error) {
	return nil, fmt.Errorf("location tracking not implemented for iOS")
}
