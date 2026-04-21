package migrations

import "embed"

// FS exposes embedded SQL migrations for runtime application setup.
//
//go:embed *.sql
var FS embed.FS
