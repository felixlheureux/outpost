// Package migrations contains the schema shipped with this executable.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
