package gpipe

import _ "embed"

// EmbeddedIndexHTML keeps the default web UI inside the server binary.
//
//go:embed dist/index.html
var EmbeddedIndexHTML []byte
