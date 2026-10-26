// Package migrations embeds versioned database migrations.
package migrations

import "embed"

// Files contains the SQL migration source.
//
//go:embed *.sql
var Files embed.FS
