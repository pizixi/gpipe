package gpipe

import "embed"

// EmbeddedWebFS keeps the default web UI directory inside the server binary.
// After the React frontend migration, static assets are built to webui/dist/.
//
//go:embed webui
var EmbeddedWebFS embed.FS
