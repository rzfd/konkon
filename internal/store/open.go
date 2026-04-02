package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Open opens a database based on driver.
//
// driver:
// - sqlite: uses dbPath
// - postgres: uses postgresDSN
func Open(ctx context.Context, driver, dbPath, postgresDSN string) (*Store, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "sqlite":
		return OpenSQLite(ctx, dbPath)
	case "postgres", "pg":
		if strings.TrimSpace(postgresDSN) == "" {
			return nil, errors.New("missing KONKON_POSTGRES_DSN")
		}
		return OpenPostgres(ctx, postgresDSN)
	default:
		return nil, fmt.Errorf("unknown DB driver: %q", driver)
	}
}

