package integration

import (
	"context"
	"fmt"
	"github.com/caarlos0/env/v6"
	"github.com/cenkalti/backoff/v4"
	"github.com/despondency/notifications-service/internal/storage"
	"github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog/log"
	"strings"
	"testing"
	"time"
)

func TestBooks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

type TestConfig struct {
	NodeHost        string `env:"NODE_HOST" envDefault:"http://localhost:8090"`
	DBConnectString string `env:"DB_CONNECT_STRING" envDefault:"postgres://root:@localhost:26257/postgres?sslmode=disable"`
	MigrationsPath  string `env:"MIGRATIONS_PATH" envDefault:"../../migrations"`
}

var (
	cfg       TestConfig
	connPool  *pgxpool.Pool
	store     Persistence
	txCreator *storage.WrappedTxCreator
)

type Persistence interface {
	Get(ctx context.Context, serverUUID uuid.UUID, tx *storage.WrappedTx) (*storage.Notification, error)
}

var _ = BeforeSuite(func() {
	SetDefaultEventuallyPollingInterval(time.Second * 1)
	SetDefaultEventuallyTimeout(time.Second * 120)

	ctx := context.Background()
	cfg = TestConfig{}
	if err := env.Parse(&cfg); err != nil {
		panic(err)
	}
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
	runMigrations(cfg.DBConnectString, cfg.MigrationsPath)

	connPool.Config().MaxConns = 3
	connPool.Config().MaxConnLifetime = time.Second * 60
	connPool.Config().MinConns = 0

	store = storage.NewCRDBPersistence(connPool)
	txCreator = storage.NewWrappedTransactionCreator(connPool)
})

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
