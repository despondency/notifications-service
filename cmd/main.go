package main

import (
	"context"
	"fmt"
	"github.com/caarlos0/env/v6"
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
}

type KafkaConfig struct {
	BootstrapServers   string `env:"KAFKA_BOOTSTRAP_SERVERS"`
	InternalGroupID    string `env:"INTERNAL_GROUP_ID"`
	InternalTopic      string `env:"INTERNAL_TOPIC"`
	OutstandingGroupID string `env:"OUTSTANDING_GROUP_ID"`
	OutstandingTopic   string `env:"OUTSTANDING_TOPIC"`
	Acks               string `env:"ACKS"`
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
	fmt.Printf("%+v\n", cfg)

	runMigrations(cfg.DBConnectString)

	connPool, err := pgxpool.Connect(ctx, cfg.DBConnectString)
	connPool.Config().MaxConns = 50
	connPool.Config().MaxConnLifetime = time.Second * 60
	connPool.Config().MinConns = 0
	if err != nil {
		log.Panic().Err(err)
	}

	pers := storage.NewCRDBPersistence(connPool)

	txCreator := storage.NewWrappedTransactionCreator(connPool)

	internalKafkaProducer, err :=
		messaging.NewKafkaProducer(cfg.BootstrapServers, cfg.InternalTopic, "notifications-service", cfg.Acks)
	if err != nil {
		log.Panic().Err(err).Msg("cannot create kafka internal producer")
	}

	outstandingKafkaProducer, err :=
		messaging.NewKafkaProducer(cfg.BootstrapServers, cfg.OutstandingTopic, "notifications-service", cfg.Acks)
	if err != nil {
		log.Panic().Err(err).Msg("cannot create kafka internal consumer")
	}

	is := notification.NewInternalService(internalKafkaProducer, outstandingKafkaProducer, txCreator, pers)

	notifiers := &notification.DelegatingNotificator{
		Notificators: []notification.Notificator{&notification.SMSNotificator{}, &notification.EmailNotificator{}, &notification.SlackNotificator{}},
	}

	ous := notification.NewOutstandingService(txCreator, pers, notifiers)

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	router := httprouter.New()
	endpoint := notification.NewEndpoint(is)
	router.POST("/notification", endpoint.CreateNotification)

	srv := http.Server{
		Addr:    ":8091",
		Handler: router,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		}
		is.Stop()
		ous.Stop()
		internalKafkaProducer.Stop()
		connPool.Close()
		close(idleConnsClosed)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatal().Err(err).Msg(fmt.Sprintf("HTTP server ListenAndServe"))
	}

	<-idleConnsClosed
}

func runMigrations(dbConnectString string) {
	crdbMigrationString := strings.Replace(dbConnectString, "postgres", "cockroachdb", 1)
	m, err := migrate.New(
		"file://./migrations",
		crdbMigrationString)
	if err != nil {
		log.Panic().Err(err).Msg("cannot create migrations")
	}
	if err := m.Up(); err != nil {
		if err.Error() == "no change" {
			return
		}
		log.Panic().Err(err).Msg("cannot run migrations")
	}
}
