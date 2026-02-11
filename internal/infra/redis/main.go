package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	client *redis.Client
}

type Config struct {
	Address  string
	Password string
	DB       int
}

func New(ctx context.Context, config Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Address,
		Password: config.Password,
		DB:       config.DB,
	})

	// Enable OpenTelemetry tracing on the Redis client.
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		return nil, err
	}

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &Client{
		client: rdb,
	}, nil
}

func (r *Client) RPush(ctx context.Context, key string, values ...any) error {
	return r.client.RPush(ctx, key, values...).Err()
}

func (r *Client) BLPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	return r.client.BLPop(ctx, timeout, keys...).Result()
}

func (r *Client) Close() error {
	return r.client.Close()
}
