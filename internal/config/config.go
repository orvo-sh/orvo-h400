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
	GitHub    GitHubConfig     `envPrefix:"GITHUB_"`
	Sandbox   SandboxConfig    `envPrefix:"SANDBOX_"`
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
	SandboxQueuePoll   string `env:"CRON_SANDBOX_QUEUE_POLL" envDefault:"*/1 * * * *"`
	AutoResolvePoll    string `env:"CRON_AUTO_RESOLVE_THRESHOLD_POLL" envDefault:"*/1 * * * *"`
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

type GitHubConfig struct {
	AppID             int64  `env:"APP_ID"`
	AppSlug           string `env:"APP_SLUG"`
	AppPrivateKey     string `env:"APP_PRIVATE_KEY"`
	AppPrivateKeyFile string `env:"APP_PRIVATE_KEY_FILE"`
	WebhookSecret     string `env:"WEBHOOK_SECRET"`
	SetupCallbackURL  string `env:"SETUP_CALLBACK_URL" envDefault:"http://localhost:8080/api/v1/github/setup/callback"`
	SetupRedirectURL  string `env:"SETUP_REDIRECT_URL" envDefault:"http://localhost:8080/settings"`
	APIBaseURL        string `env:"API_BASE_URL" envDefault:"https://api.github.com"`
	AppBaseURL        string `env:"APP_BASE_URL" envDefault:"https://github.com"`
	StateSecret       string `env:"STATE_SECRET"`
}

type SandboxConfig struct {
	DockerBinary        string        `env:"DOCKER_BINARY" envDefault:"docker"`
	DefaultImage        string        `env:"DEFAULT_IMAGE" envDefault:"mirror.gcr.io/library/node:25"`
	WorkingDir          string        `env:"WORKING_DIR" envDefault:"/workspace"`
	CPULimit            string        `env:"CPU_LIMIT" envDefault:"2"`
	MemoryLimit         string        `env:"MEMORY_LIMIT" envDefault:"4g"`
	FallbackToContainer bool          `env:"FALLBACK_TO_CONTAINER" envDefault:"true"`
	JobTimeout          time.Duration `env:"JOB_TIMEOUT" envDefault:"45m"`
	CommandTimeout      time.Duration `env:"COMMAND_TIMEOUT" envDefault:"10m"`
	BootstrapTimeout    time.Duration `env:"BOOTSTRAP_TIMEOUT" envDefault:"120s"`
	GitAuthorName       string        `env:"GIT_AUTHOR_NAME" envDefault:"orvo-bot"`
	GitAuthorEmail      string        `env:"GIT_AUTHOR_EMAIL" envDefault:"orvo-bot@users.noreply.github.com"`
	OpencodeCommand     string        `env:"OPENCODE_COMMAND" envDefault:"opencode"`
	OpencodeModel       string        `env:"OPENCODE_MODEL"`
	OpencodeVariant     string        `env:"OPENCODE_VARIANT"`
	OpencodeAgent       string        `env:"OPENCODE_AGENT"`
	OpencodeTimeout     time.Duration `env:"OPENCODE_TIMEOUT" envDefault:"8m"`
	AutoResolveFastPath bool          `env:"AUTO_RESOLVE_FAST_PATH" envDefault:"false"`
	ImagePrepullEnabled bool          `env:"IMAGE_PREPULL_ENABLED" envDefault:"true"`
	ImagePrepullTimeout time.Duration `env:"IMAGE_PREPULL_TIMEOUT" envDefault:"120s"`
}

func Load() (*Config, error) {
	var cfg Config
	err := env.Parse(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse environment variables, %w", err)
	}

	return &cfg, nil
}
