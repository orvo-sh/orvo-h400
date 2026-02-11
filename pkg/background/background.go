package background

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

type Config struct {
	DefaultTimeout time.Duration
}

type Manager struct {
	logger *slog.Logger
	config Config
	wg     sync.WaitGroup
}

func New(logger *slog.Logger, config Config) *Manager {
	return &Manager{
		logger: logger,
		config: config,
	}
}

func (m *Manager) Run(fn func(ctx context.Context)) {
	m.wg.Add(1)

	go func() {
		defer m.wg.Done()

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				m.logger.Error("background task panicked",
					slog.Any("panic", r),
					slog.String("stack", stack),
				)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), m.config.DefaultTimeout)
		defer cancel()

		fn(ctx)
	}()
}

func (m *Manager) Wait() {
	m.wg.Wait()
}
