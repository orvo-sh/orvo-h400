package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig      `envPrefix:"APP_"`
	Redis    RedisConfig    `envPrefix:"REDIS_"`
	Postgres PostgresConfig `envPrefix:"POSTGRES_"`
}

type RedisConfig struct {
	Address  string `env:"ADDRESS"`
	Password string `env:"PASSWORD"`
	DB       int    `env:"DB"`
}

type PostgresConfig struct {
	URL string `env:"URL"` // e.g., postgres://user:pass@localhost:5432/dbname?sslmode=disable
}

type AppConfig struct {
	Environment string `env:"ENVIRONMENT"`
	IngestPort  string `env:"INGEST_PORT"`
	AppPort     string `env:"APP_PORT"`
}

func Load() (*Config, error) {
	err := godotenv.Load(".env")
	if err != nil {
		return nil, fmt.Errorf("failed to load .env file, %w", err)
	}

	var cfg Config
	err = env.Parse(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse environment variables, %w", err)
	}

	return &cfg, nil
}
