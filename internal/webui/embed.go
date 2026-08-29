// Package webui serves the embedded SPA. The real frontend build output lands
// in dist/ (vite outDir, see web/vite.config.ts); a fresh git checkout only has
// dist/.gitkeep, in which case FS falls back to the placeholder page so the Go
// binary always compiles and serves a meaningful answer at /.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist all:placeholder
var content embed.FS

// FS returns the SPA filesystem: dist/ when it contains a real build
// (index.html present), otherwise the placeholder.
func FS() fs.FS {
	if dist, err := fs.Sub(content, "dist"); err == nil {
		if f, err := dist.Open("index.html"); err == nil {
			_ = f.Close()
			return dist
		}
	}
	placeholder, err := fs.Sub(content, "placeholder")
	if err != nil {
		panic("webui: placeholder FS missing: " + err.Error())
	}
	return placeholder
}
