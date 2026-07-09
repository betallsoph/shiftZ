// Command migratediff (re)generates versioned Atlas migrations in
// ./migrations by diffing the ent schemas against their replayed state on a
// clean dev database.
//
// Usage:
//
//	DEV_DATABASE_URL='postgres://user@localhost:5433/dev?sslmode=disable' \
//	  go run ./cmd/migratediff <migration-name>
//
// The dev database is a scratch database Atlas uses for computing the diff;
// it is wiped between runs and must not be your real database.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	atlas "ariga.io/atlas/sql/migrate"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/betallsoph/shiftz/internal/ent/migrate"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migratediff:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: migratediff <migration-name>")
	}
	devURL := os.Getenv("DEV_DATABASE_URL")
	if devURL == "" {
		return fmt.Errorf("DEV_DATABASE_URL is required (scratch database for computing the diff)")
	}

	dir, err := atlas.NewLocalDir("migrations")
	if err != nil {
		return fmt.Errorf("open migrations dir: %w", err)
	}
	db, err := sql.Open("pgx", devURL)
	if err != nil {
		return fmt.Errorf("open dev database: %w", err)
	}
	defer db.Close()

	drv := entsql.OpenDB(dialect.Postgres, db)
	return migrate.NewSchema(drv).NamedDiff(context.Background(), os.Args[1],
		schema.WithDir(dir),
		schema.WithMigrationMode(schema.ModeReplay),
		schema.WithDialect(dialect.Postgres),
		schema.WithFormatter(atlas.DefaultFormatter),
		schema.WithDropColumn(true),
		schema.WithDropIndex(true),
	)
}
