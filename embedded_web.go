package gpipe

import "embed"

// EmbeddedWebFS keeps the default web UI directory inside the server binary.
// React static assets are built to frontend/dist/.
//
//go:embed frontend/dist
var EmbeddedWebFS embed.FS
