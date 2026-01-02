//go:build linux

package platforms

import "discord-c2-poc/cmd/agent/platforms/linux"

func newPlatform() Platform {
	return linux.NewPlatform()
}
