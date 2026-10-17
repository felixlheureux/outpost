//go:build !dev

// Package web embeds the production SPA. Build web/dist before compiling Go.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

func Assets() fs.FS {
	assets, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return assets
}
