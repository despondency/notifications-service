package main

import (
	"fmt"
	"github.com/caarlos0/env/v6"
	"github.com/despondency/notifications-service/internal/notification"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/cockroachdb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Config struct {
	KafkaConfig
	DBConfig
}

type KafkaConfig struct {
}

type DBConfig struct {
	DBConnectString string `env:"DB_CONNECT_STRING"`
}

func main() {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", cfg)

	runMigrations()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	router := httprouter.New()
	endpoint := notification.Endpoint{}
	router.POST("/notification", endpoint.CreateNotification)
}

func runMigrations() {
	m, err := migrate.New(
		"file://./migrations",
		"cockroachdb://cockroach:@localhost:26257/postgresql?sslmode=disable")
	if err != nil {
		log.Fatal().Err(err)
	}
	if err := m.Up(); err != nil {
		log.Fatal().Err(err)
	}
}
