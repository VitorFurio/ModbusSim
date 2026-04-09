package frontend

import (
	"embed"
	"io/fs"
)

//go:embed dist
var files embed.FS

// FS returns the embedded frontend filesystem (the dist directory).
func FS() fs.FS {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		return nil
	}
	return sub
}
