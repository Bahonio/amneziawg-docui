// Package web contains the AWG DocUI browser application.
package web

import (
	"embed"
	"io/fs"
)

// static contains the complete frontend, including its local font files.
// Embedding keeps the server binary and container image self-contained.
//
//go:embed static
var static embed.FS

// Assets returns the root served at /. The error is impossible because the
// embedded directory is checked at compile time.
func Assets() fs.FS {
	root, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}
	return root
}
