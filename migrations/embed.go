// Package migrations embeds the SQL migration files into the binary.
// Importing this package from cmd/api/main.go ensures the correct migration
// version always ships with the binary — no file path dependency at runtime.
package migrations

import "embed"

// FS contains all *.sql migration files in this directory.
// golang-migrate's iofs source driver reads from this embedded filesystem.
//
//go:embed *.sql
var FS embed.FS
