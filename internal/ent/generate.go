// Package ent hosts the generated ent client for shiftbot's data layer.
// Run `go generate ./...` after changing anything in schema/.
package ent

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate --feature sql/versioned-migration,sql/upsert ./schema
