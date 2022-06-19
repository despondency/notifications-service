package test

type Config struct {
	NodeHost        string `env:"NODE_HOST" envDefault:"http://localhost:8090"`
	DBConnectString string `env:"DB_CONNECT_STRING" envDefault:"postgres://root:@localhost:26257/postgres?sslmode=disable"`
	MigrationsPath  string `env:"MIGRATIONS_PATH" envDefault:"../../../migrations"`
}
