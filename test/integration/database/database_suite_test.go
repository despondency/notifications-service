package database_test

import (
	"context"
	"github.com/caarlos0/env/v6"
	"github.com/despondency/notifications-service/internal/notification"
	"github.com/despondency/notifications-service/test"
	"github.com/jackc/pgx/v4/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestDatabase(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Database")
}

var (
	cfg      *test.Config
	connPool *pgxpool.Pool
	store    *notification.CRDBPersistence
)

var _ = BeforeSuite(func() {
	ctx := context.Background()
	cfg = &test.Config{}
	if err := env.Parse(cfg); err != nil {
		panic(err)
	}
	connPool, store = test.SetupDatabase(cfg, ctx)
})
