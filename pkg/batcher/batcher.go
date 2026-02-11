package batcher

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

type WriterFunc[T any] func(ctx context.Context, batch []T) error

type Option func(*options)

type options struct {
	batchSize     int
	flushInterval time.Duration
	maxQueueSize  int
}

func WithBatchSize(size int) Option {
	return func(o *options) { o.batchSize = size }
}

func WithFlushInterval(interval time.Duration) Option {
	return func(o *options) { o.flushInterval = interval }
}

func WithMaxQueueSize(size int) Option {
	return func(o *options) { o.maxQueueSize = size }
}

type Batcher[T any] struct {
	logger  *slog.Logger
	options options
	input   chan T
	writeFn WriterFunc[T]

	done chan struct{}
	wg   sync.WaitGroup
	once sync.Once
}

func New[T any](logger *slog.Logger, writer WriterFunc[T], opts ...Option) *Batcher[T] {
	o := options{
		batchSize:     1000,
		flushInterval: 5 * time.Second,
		maxQueueSize:  10000,
	}

	for _, set := range opts {
		set(&o)
	}

	b := &Batcher[T]{
		logger:  logger,
		options: o,
		input:   make(chan T, o.maxQueueSize),
		writeFn: writer,
		done:    make(chan struct{}),
	}

	b.wg.Add(1)
	go b.loop()
	return b
}

func (b *Batcher[T]) Push(item T) error {
	select {
	case <-b.done:
		return fmt.Errorf("batcher is closed")
	case b.input <- item:
		return nil
	default:
		return fmt.Errorf("queue full, dropping item")
	}
}

func (b *Batcher[T]) Close() {
	b.once.Do(func() {
		close(b.done)
		b.wg.Wait()
	})
}

func (b *Batcher[T]) loop() {
	defer b.wg.Done()

	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			b.logger.Error("loop: batcher loop panicked",
				slog.Any("panic", r),
				slog.String("stack", stack),
			)
		}
	}()

	ticker := time.NewTicker(b.options.flushInterval)
	defer ticker.Stop()

	batch := make([]T, 0, b.options.batchSize)

	flush := func() {
		count := len(batch)
		if count == 0 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := b.writeFn(ctx, batch); err != nil {
			b.logger.Error("loop: batcher flush failed",
				slog.Int("count", count),
				slog.Any("error", err),
			)
		} else {
			b.logger.Debug("loop: batcher flushed", slog.Int("count", count))
		}
		batch = batch[:0]
	}

	defer func() {
		for {
			select {
			case item := <-b.input:
				batch = append(batch, item)
			default:
				if len(batch) > 0 {
					b.logger.Info("loop: flushing remaining items on shutdown", slog.Int("count", len(batch)))
					flush()
				}
				return
			}
		}
	}()

	for {
		select {
		case item := <-b.input:
			batch = append(batch, item)
			if len(batch) >= b.options.batchSize {
				flush()
				ticker.Reset(b.options.flushInterval)
			}

		case <-ticker.C:
			flush()

		case <-b.done:
			return
		}
	}
}
