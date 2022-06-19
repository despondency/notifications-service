package notifications_test

import (
	"context"
	"github.com/caarlos0/env/v6"
	"github.com/despondency/notifications-service/internal/storage"
	"github.com/despondency/notifications-service/test"
	"github.com/jackc/pgx/v4/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
	"time"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var (
	cfg      *test.Config
	connPool *pgxpool.Pool
	store    *storage.CRDBPersistence
)

var _ = BeforeSuite(func() {
	SetDefaultEventuallyPollingInterval(time.Second * 1)
	SetDefaultEventuallyTimeout(time.Second * 120)

	ctx := context.Background()
	cfg = &test.Config{}
	if err := env.Parse(cfg); err != nil {
		panic(err)
	}
	connPool, store = test.SetupDatabase(cfg, ctx)
})
