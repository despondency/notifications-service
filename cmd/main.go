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
	"strings"
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

	internalKafkaConsumer, err :=
		messaging.NewKafkaConsumer(cfg.BootstrapServers, cfg.InternalGroupID, cfg.InternalTopic, "earliest")
	if err != nil {
		log.Panic().Err(err).Msg("cannot create kafka internal consumer")
	}

	outstandingKafkaProducer, err :=
		messaging.NewKafkaProducer(cfg.BootstrapServers, cfg.OutstandingTopic, "notifications-service", cfg.Acks)
	if err != nil {
		log.Panic().Err(err).Msg("cannot create kafka internal consumer")
	}

	is := notification.NewInternalService(internalKafkaProducer, internalKafkaConsumer, outstandingKafkaProducer, txCreator, pers)

	outstandingKafkaConsumer, err :=
		messaging.NewKafkaConsumer(cfg.BootstrapServers, cfg.OutstandingGroupID, cfg.OutstandingTopic, "latest")
	if err != nil {
		log.Panic().Err(err).Msg("cannot create kafka internal consumer")
	}

	notifiers := &notification.DelegatingNotificator{
		Notificators: []notification.Notificator{&notification.SMSNotificator{}, &notification.EmailNotificator{}, &notification.SlackNotificator{}},
	}

	_ = notification.NewOutstandingService(outstandingKafkaConsumer, txCreator, pers, notifiers)

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	router := httprouter.New()
	endpoint := notification.NewEndpoint(is)
	router.POST("/notification", endpoint.CreateNotification)

	err = http.ListenAndServe(":8091", router)
	log.Fatal().Err(err)
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
