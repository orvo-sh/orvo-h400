package workers

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
)

type ManagerConfig struct {
	DefaultTimeout time.Duration
	Timezone       *time.Location
}

type Manager struct {
	logger *slog.Logger
	pool   *pgxpool.Pool
	config ManagerConfig

	cron *cron.Cron

	mu      sync.Mutex
	running bool
	done    chan struct{}
	wg      sync.WaitGroup
}

type jobConfig struct {
	Name    string
	Timeout time.Duration
	LockKey int64
	Run     func(ctx context.Context) error
}

func NewManager(logger *slog.Logger, pool *pgxpool.Pool, config ManagerConfig) *Manager {
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Second
	}
	if config.Timezone == nil {
		config.Timezone = time.UTC
	}

	c := cron.New(
		cron.WithLocation(config.Timezone),
		cron.WithParser(cron.NewParser(cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow)),
		cron.WithChain(cron.Recover(cron.DefaultLogger)),
	)

	return &Manager{
		logger: logger.With("module", "worker_manager"),
		pool:   pool,
		config: config,
		cron:   c,
		done:   make(chan struct{}),
	}
}

func (m *Manager) RegisterCron(name string, expression string, run func(ctx context.Context) error, timeout ...time.Duration) error {
	cfg := jobConfig{
		Name: name,
		Run:  run,
	}
	if len(timeout) > 0 && timeout[0] > 0 {
		cfg.Timeout = timeout[0]
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = m.config.DefaultTimeout
	}
	cfg.LockKey = advisoryLockKey(name)

	_, err := m.cron.AddFunc(expression, func() {
		m.runJob(cfg)
	})
	if err != nil {
		return fmt.Errorf("register cron %s: %w", name, err)
	}

	m.logger.Info("registered cron worker", slog.String("name", name), slog.String("cron", expression))
	return nil
}

func (m *Manager) RegisterChannel(name string, trigger <-chan struct{}, run func(ctx context.Context) error, timeout ...time.Duration) {
	cfg := jobConfig{
		Name: name,
		Run:  run,
	}
	if len(timeout) > 0 && timeout[0] > 0 {
		cfg.Timeout = timeout[0]
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = m.config.DefaultTimeout
	}
	cfg.LockKey = advisoryLockKey(name)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-m.done:
				return
			case <-trigger:
				m.runJob(cfg)
			}
		}
	}()

	m.logger.Info("registered channel worker", slog.String("name", name))
}

func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return
	}
	m.cron.Start()
	m.running = true
	m.logger.Info("worker manager started")
}

func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	close(m.done)
	stopCtx := m.cron.Stop()
	m.running = false
	m.mu.Unlock()

	<-stopCtx.Done()
	m.wg.Wait()
	m.logger.Info("worker manager stopped")
}

func (m *Manager) runJob(cfg jobConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	locked, err := m.tryAdvisoryLock(ctx, cfg.LockKey)
	if err != nil {
		m.logger.Error("failed to acquire worker lock",
			slog.String("name", cfg.Name),
			slog.Int64("lock_key", cfg.LockKey),
			slog.Any("error", err),
		)
		return
	}
	if !locked {
		m.logger.Debug("worker lock held by another instance",
			slog.String("name", cfg.Name),
			slog.Int64("lock_key", cfg.LockKey),
		)
		return
	}
	defer m.unlockAdvisoryLock(context.Background(), cfg.LockKey)

	start := time.Now()
	if err := cfg.Run(ctx); err != nil {
		m.logger.Error("worker run failed",
			slog.String("name", cfg.Name),
			slog.Duration("duration", time.Since(start)),
			slog.Any("error", err),
		)
		return
	}

	m.logger.Debug("worker run completed",
		slog.String("name", cfg.Name),
		slog.Duration("duration", time.Since(start)),
	)
}

func (m *Manager) tryAdvisoryLock(ctx context.Context, key int64) (bool, error) {
	var locked bool
	if err := m.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked); err != nil {
		return false, err
	}
	return locked, nil
}

func (m *Manager) unlockAdvisoryLock(ctx context.Context, key int64) {
	if _, err := m.pool.Exec(ctx, "SELECT pg_advisory_unlock($1)", key); err != nil {
		m.logger.Warn("failed to unlock worker advisory lock",
			slog.Int64("lock_key", key),
			slog.Any("error", err),
		)
	}
}

func advisoryLockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return int64(h.Sum64())
}
