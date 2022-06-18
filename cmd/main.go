package main

import (
	"context"
	"fmt"
	"github.com/caarlos0/env/v6"
	"github.com/cenkalti/backoff/v4"
	"github.com/despondency/notifications-service/internal/messaging"
	"github.com/despondency/notifications-service/internal/notification"
	"github.com/despondency/notifications-service/internal/storage"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/cockroachdb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

type Config struct {
	KafkaConfig
	DBConfig
	Port int `env:"PORT" envDefault:"8090"`
}

type KafkaConfig struct {
	BootstrapServers   string `env:"KAFKA_BOOTSTRAP_SERVERS"`
	OutstandingGroupID string `env:"OUTSTANDING_GROUP_ID"`
	OutstandingTopic   string `env:"OUTSTANDING_TOPIC"`
}

type DBConfig struct {
	DBConnectString string `env:"DB_CONNECT_STRING"`
	MigrationsPath  string `env:"MIGRATIONS_PATH"`
}

func main() {
	ctx := context.Background()
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		panic(err)
	}

	runMigrations(cfg.DBConnectString)

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
	connPool.Config().MaxConns = 50
	connPool.Config().MaxConnLifetime = time.Second * 60
	connPool.Config().MinConns = 0

	pers := storage.NewCRDBPersistence(connPool)

	txCreator := storage.NewWrappedTransactionCreator(connPool)

	outstandingKafkaProducer, err :=
		messaging.NewKafkaProducer(cfg.BootstrapServers, cfg.OutstandingTopic, "notifications-service", "1")
	if err != nil {
		log.Panic().Err(err).Msg("cannot create kafka internal consumer")
	}

	is := notification.NewInternalService(outstandingKafkaProducer, txCreator, pers)

	notifiers := &notification.DelegatingNotificator{
		Notificators: []notification.Notificator{&notification.SMSNotificator{}, &notification.EmailNotificator{}, &notification.SlackNotificator{}},
	}

	ous := notification.NewOutstandingService(cfg.BootstrapServers, cfg.OutstandingGroupID, "earliest",
		"true", cfg.OutstandingTopic,
		txCreator, pers, notifiers)

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	router := httprouter.New()
	endpoint := notification.NewEndpoint(is)
	router.POST("/notification", endpoint.CreateNotification)

	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: router,
	}

	wait := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		}
		ous.Stop()
		connPool.Close()
		close(wait)
	}()

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// Error starting or closing listener:
			log.Fatal().Err(err).Msg(fmt.Sprintf("HTTP server ListenAndServe"))
		}
	}()
	log.Info().Msg(fmt.Sprintf("notifications service started at port %d", cfg.Port))

	<-wait
}

func runMigrations(dbConnectString string) {
	crdbMigrationString := strings.Replace(dbConnectString, "postgres", "cockroachdb", 1)
	err := backoff.Retry(func() error {
		m, err := migrate.New(
			"file://./migrations",
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
