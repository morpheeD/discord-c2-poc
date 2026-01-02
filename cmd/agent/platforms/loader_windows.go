//go:build windows

package platforms

import "discord-c2-poc/cmd/agent/platforms/windows"

func newPlatform() Platform {
	return windows.NewPlatform()
}
