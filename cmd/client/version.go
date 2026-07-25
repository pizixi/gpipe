package main

import "runtime"

// These values are overridden by release builds. Keeping useful defaults also
// makes locally-built development clients identifiable and upgrade-capable.
var clientVersion = "1.0.0"
var clientPlatform string

func effectiveClientPlatform() string {
	if clientPlatform != "" {
		return clientPlatform
	}
	return runtime.GOOS + "-" + runtime.GOARCH
}
