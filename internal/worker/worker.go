package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/orvo-sh/orvo/internal/domain/models"
	clickhousedb "github.com/orvo-sh/orvo/internal/infra/clickhouse-db"
	infraredis "github.com/orvo-sh/orvo/internal/infra/redis"
)

type Worker struct {
	redisClient *infraredis.Client
	chClient    *clickhousedb.DB
	batchSize   int
	flushInterval time.Duration
}

func New(redisClient *infraredis.Client, chClient *clickhousedb.DB) *Worker {
	return &Worker{
		redisClient: redisClient,
		chClient:    chClient,
		batchSize:   1000,
		flushInterval: 5 * time.Second,
	}
}

func (w *Worker) Start(ctx context.Context) {
	fmt.Println("Worker started...")
	buffer := make([]models.Log, 0, w.batchSize)
	lastFlush := time.Now()

	for {
		select {
		case <-ctx.Done():
			if len(buffer) > 0 {
				w.flush(context.Background(), buffer)
			}
			return
		default:
			res, err := w.redisClient.BLPop(ctx, 1*time.Second, "logs_queue")
			
			if err == nil && len(res) >= 2 {
				var l models.Log
				if err := json.Unmarshal([]byte(res[1]), &l); err == nil {
					buffer = append(buffer, l)
				} else {
					log.Printf("Failed to unmarshal log: %v", err)
				}
			} else if err != nil && err != redis.Nil {
                if ctx.Err() == nil {
				    log.Printf("Redis error: %v", err)
                }
			}

			if len(buffer) >= w.batchSize || time.Since(lastFlush) >= w.flushInterval {
				if len(buffer) > 0 {
					if err := w.flush(ctx, buffer); err != nil {
						log.Printf("Failed to flush buffer: %v", err)
					}
					buffer = buffer[:0]
					lastFlush = time.Now()
				}
			}
		}
	}
}

func (w *Worker) flush(ctx context.Context, logs []models.Log) error {
	batch, err := w.chClient.PrepareBatch(ctx, "INSERT INTO logs (id, timestamp, level, message, service, environment, organization_id, trace_id, span_id, parent_id, attributes)")
	if err != nil {
		return err
	}

	for _, l := range logs {
		ts, err := time.Parse(time.RFC3339, l.Timestamp)
		if err != nil {
			ts = time.Now()
		}
        
        attrs, _ := json.Marshal(l.Attributes)

		err = batch.Append(
			l.ID,
			ts,
			l.Level,
			l.Message,
			l.Service,
			l.Environment,
			l.OrganizationID,
			l.TraceID,
			l.SpanID,
			l.ParentID,
			string(attrs),
		)
		if err != nil {
			return err
		}
	}

	return batch.Send()
}
