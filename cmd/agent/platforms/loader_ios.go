//go:build ios

package platforms

import "discord-c2-poc/cmd/agent/platforms/ios"

func newPlatform() Platform {
	return ios.NewPlatform()
}
