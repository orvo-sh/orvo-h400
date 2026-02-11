package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

type Config struct {
	ServiceName  string
	Environment  string
	OTLPEndpoint string
	APIKey       string

	// Resource is an optional pre-built OTEL resource. When provided the
	// logger will reuse it instead of creating its own, so the same
	// service.name / deployment.environment attributes are shared with the
	// TracerProvider.
	Resource *resource.Resource
}

func New(ctx context.Context, config Config) (*slog.Logger, func(), error) {
	res := config.Resource
	if res == nil {
		// Backwards-compatible: if no resource was supplied, build one.
		var err error
		res, err = resource.New(ctx, resource.WithFromEnv())
		if err != nil {
			return nil, nil, err
		}
	}

	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(config.OTLPEndpoint),
		otlploggrpc.WithInsecure(),
		otlploggrpc.WithHeaders(map[string]string{
			"x-api-key": config.APIKey,
		}),
	)
	if err != nil {
		return nil, nil, err
	}

	processor := log.NewBatchProcessor(exporter)
	provider := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(processor),
	)

	global.SetLoggerProvider(provider)

	otelHandler := &jsonWrapperHandler{Handler: otelslog.NewHandler(config.ServiceName)}

	var handler slog.Handler
	if config.Environment == "production" {
		handler = otelHandler
	} else {
		tintHandler := tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelDebug})
		handler = &teeHandler{
			handlers: []slog.Handler{tintHandler, otelHandler},
		}
	}

	logger := slog.New(&contextHandler{handler})

	cleanup := func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			slog.Error("failed to shutdown logger provider", "error", err)
		}
	}

	return logger, cleanup, nil
}
