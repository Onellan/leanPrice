// Package migrations exposes the immutable forward-only SQL migration files.
package migrations

import "embed"

// Files is the checked-in migration set used by the runtime.
//
//go:embed *.sql
var Files embed.FS
