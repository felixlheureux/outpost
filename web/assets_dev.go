//go:build dev

package web

import "io/fs"

// Assets is absent during development: Vite serves the UI and proxies /api.
func Assets() fs.FS { return nil }
