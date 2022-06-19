package test

import (
	"context"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/despondency/notifications-service/internal/storage"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/cockroachdb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog/log"
	"strings"
	"time"
)

func SetupDatabase(cfg *Config, ctx context.Context) (*pgxpool.Pool, *storage.CRDBPersistence) {
	var connPool *pgxpool.Pool
	err := backoff.Retry(func() error {
		var err error
		connPool, err = pgxpool.Connect(ctx, cfg.DBConnectString)
		if err != nil {
			return err
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 10))
	if err != nil {
		log.Panic().Err(err).Msg("cannot connect")
	}
	connPool.Config().MaxConns = 3
	connPool.Config().MaxConnLifetime = time.Second * 60
	connPool.Config().MinConns = 0

	runMigrations(cfg.DBConnectString, cfg.MigrationsPath)

	store := storage.NewCRDBPersistence(connPool)
	return connPool, store
}

func runMigrations(dbConnectString, migrationPath string) {
	crdbMigrationString := strings.Replace(dbConnectString, "postgres", "cockroachdb", 1)
	err := backoff.Retry(func() error {
		m, err := migrate.New(
			fmt.Sprintf("%s%s", "file://", migrationPath),
			crdbMigrationString)
		if err != nil {
			return err
		}
		if err = m.Up(); err != nil {
			if err.Error() == "no change" {
				return nil
			}
			return err
		}
		log.Info().Msg("migrations ran successfully")
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 10))
	if err != nil {
		log.Panic().Err(err).Msg("cannot run migrations")
	}
}
