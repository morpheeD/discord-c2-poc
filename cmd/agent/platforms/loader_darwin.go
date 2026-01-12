//go:build darwin && !ios

package platforms

import "discord-c2-poc/cmd/agent/platforms/darwin"

func newPlatform() Platform {
	return darwin.NewPlatform()
}
