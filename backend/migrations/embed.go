// Package migrations embeds the SQL schema files so the server binary can
// migrate its own database on startup — no external migrate CLI needed on the
// VPS or inside the container image.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
