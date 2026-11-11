// Package webui embeds the compiled React app (web/dist, copied here by `make web`).
package webui

import (
	"embed"
	"io/fs"
)

// The "all:" prefix includes dotfiles so the .gitkeep placeholder keeps the
// directory embeddable before the UI has been built.
//
//go:embed all:dist
var dist embed.FS

// FS returns the UI files rooted at the dist directory.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // cannot happen: "dist" is embedded above
	}
	return sub
}
