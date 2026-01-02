//go:build darwin

package platforms

import "discord-c2-poc/cmd/agent/platforms/darwin"

func newPlatform() Platform {
	return darwin.NewPlatform()
}
