package postgres

import (
	"context"
	"fmt"

	"github.com/AndyBarnv/golang-project-andybarnv/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrationDB := stdlib.OpenDBFromPool(pool)
	defer migrationDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, migrationDB, migrations.Files)
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
