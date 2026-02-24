package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	App       AppConfig        `envPrefix:"APP_"`
	Postgres  PostgresConfig   `envPrefix:"POSTGRES_"`
	Session   SessionConfig    `envPrefix:"SESSION_"`
	Otel      OtelConfig       `envPrefix:"OTEL_"`
	Telemetry TelemetryConfig  `envPrefix:"TELEMETRY_"`
	Workers   WorkerCronConfig `envPrefix:"WORKER_"`
	S3        S3ArchiveConfig  `envPrefix:"S3_"`
}

type PostgresConfig struct {
	URL string `env:"URL"`
}

type OtelConfig struct {
	Endpoint string `env:"ENDPOINT"`
	ApiKey   string `env:"API_KEY"`
}

type SessionConfig struct {
	CookieName string `env:"COOKIE_NAME"`
}

type AppConfig struct {
	Environment    string `env:"ENVIRONMENT" envDefault:"development"`
	AppPort        string `env:"APP_PORT" envDefault:"8080"`
	IngestPort     string `env:"INGEST_PORT"`
	OtlpGRPCPort   string `env:"OTLP_GRPC_PORT" envDefault:"4317"`
	OtlpHTTPPort   string `env:"OTLP_HTTP_PORT" envDefault:"4318"`
	EnableAPI      bool   `env:"ENABLE_API" envDefault:"true"`
	EnableOTLPGRPC bool   `env:"ENABLE_OTLP_GRPC" envDefault:"true"`
	EnableOTLPHTTP bool   `env:"ENABLE_OTLP_HTTP" envDefault:"true"`
	EnableWorkers  bool   `env:"ENABLE_WORKERS" envDefault:"true"`
}

type TelemetryConfig struct {
	PartitionPrecreateDays  int           `env:"PARTITION_PRECREATE_DAYS" envDefault:"7"`
	HotRetentionLogs        time.Duration `env:"HOT_RETENTION_LOGS" envDefault:"168h"`
	HotRetentionTraces      time.Duration `env:"HOT_RETENTION_TRACES" envDefault:"168h"`
	HotRetentionMetrics     time.Duration `env:"HOT_RETENTION_METRICS" envDefault:"168h"`
	ArchiveRetentionLogs    time.Duration `env:"ARCHIVE_RETENTION_LOGS" envDefault:"8760h"`
	ArchiveRetentionTraces  time.Duration `env:"ARCHIVE_RETENTION_TRACES" envDefault:"8760h"`
	ArchiveRetentionMetrics time.Duration `env:"ARCHIVE_RETENTION_METRICS" envDefault:"17520h"`
	RestoredTTL             time.Duration `env:"RESTORED_TTL" envDefault:"24h"`
	RestoreThroughputBPS    int64         `env:"RESTORE_THROUGHPUT_BPS" envDefault:"26214400"`
}

type WorkerCronConfig struct {
	PartitionPrecreate string `env:"CRON_PARTITION_PRECREATE" envDefault:"0 0 * * *"`
	HotRetention       string `env:"CRON_HOT_RETENTION" envDefault:"30 0 * * *"`
	ArchiveExport      string `env:"CRON_ARCHIVE_EXPORT" envDefault:"15 0 * * *"`
	ArchiveRetention   string `env:"CRON_ARCHIVE_RETENTION" envDefault:"45 0 * * *"`
	RestoreTTL         string `env:"CRON_RESTORE_TTL" envDefault:"*/30 * * * *"`
	RestoreQueuePoll   string `env:"CRON_RESTORE_QUEUE_POLL" envDefault:"*/1 * * * *"`
}

type S3ArchiveConfig struct {
	Bucket       string `env:"BUCKET"`
	Region       string `env:"REGION"`
	Endpoint     string `env:"ENDPOINT"`
	Prefix       string `env:"PREFIX" envDefault:"telemetry"`
	AccessKeyID  string `env:"ACCESS_KEY_ID"`
	SecretKey    string `env:"SECRET_ACCESS_KEY"`
	UsePathStyle bool   `env:"USE_PATH_STYLE" envDefault:"false"`
}

func Load() (*Config, error) {
	var cfg Config
	err := env.Parse(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse environment variables, %w", err)
	}

	return &cfg, nil
}
