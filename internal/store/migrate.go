package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func runMigrations(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create _migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		var count int
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM _migrations WHERE name = ?`, name).Scan(&count)
		if count > 0 {
			continue
		}
		b, err := migrationFS.ReadFile(path.Join("migrations", name))
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, string(b)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		_, _ = db.ExecContext(ctx, `INSERT INTO _migrations (name, applied_at) VALUES (?, ?)`, name, time.Now().UTC().Format(time.RFC3339))
	}
	return nil
}
