package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// OpenPostgres opens a Postgres database, applies migrations, and returns a Store.
func OpenPostgres(ctx context.Context, postgresDSN string) (*Store, error) {
	if postgresDSN == "" {
		return nil, errors.New("missing postgresDSN")
	}

	db, err := sql.Open("pgx", postgresDSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := runMigrationsPostgres(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{
		db: db,
		rebind: func(query string) string {
			return sqlx.Rebind(sqlx.DOLLAR, query)
		},
	}, nil
}

