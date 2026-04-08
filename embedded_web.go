package gpipe

import "embed"

// EmbeddedWebFS keeps the default web UI directory inside the server binary.
//
//go:embed webui
var EmbeddedWebFS embed.FS
