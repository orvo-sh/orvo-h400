package workers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PartitionManager struct {
	logger *slog.Logger
	pool   *pgxpool.Pool
}

func NewPartitionManager(logger *slog.Logger, pool *pgxpool.Pool) *PartitionManager {
	return &PartitionManager{
		logger: logger.With("module", "partition_manager"),
		pool:   pool,
	}
}

func (m *PartitionManager) EnsureFuturePartitions(ctx context.Context, days int) error {
	if days < 0 {
		days = 0
	}

	for offset := 0; offset <= days; offset++ {
		day := time.Now().UTC().AddDate(0, 0, offset)
		if err := m.EnsurePartitionsForDay(ctx, day); err != nil {
			return err
		}
	}
	return nil
}

func (m *PartitionManager) EnsurePartitionsForDay(ctx context.Context, day time.Time) error {
	dayStart := truncateUTCDate(day)
	dayEnd := dayStart.AddDate(0, 0, 1)
	suffix := dayStart.Format("20060102")

	tables := []string{
		"logs_hot",
		"logs_restored",
		"traces_hot",
		"traces_restored",
		"metrics_hot",
		"metrics_restored",
	}

	for _, table := range tables {
		partitionName := fmt.Sprintf("%s_p%s", table, suffix)
		dayStartLiteral := sqlTimestampLiteral(dayStart)
		dayEndLiteral := sqlTimestampLiteral(dayEnd)
		query := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM (%s) TO (%s)",
			partitionName,
			table,
			dayStartLiteral,
			dayEndLiteral,
		)
		if _, err := m.pool.Exec(ctx, query); err != nil {
			return fmt.Errorf("create partition %s: %w", partitionName, err)
		}
	}

	return nil
}

func (m *PartitionManager) ApplyHotRetention(
	ctx context.Context,
	logsRetention time.Duration,
	tracesRetention time.Duration,
	metricsRetention time.Duration,
) error {
	if err := m.dropOldPartitions(ctx, "logs_hot", logsRetention); err != nil {
		return err
	}
	if err := m.dropOldPartitions(ctx, "traces_hot", tracesRetention); err != nil {
		return err
	}
	if err := m.dropOldPartitions(ctx, "metrics_hot", metricsRetention); err != nil {
		return err
	}
	return nil
}

func (m *PartitionManager) ApplyRestoredTTL(ctx context.Context, ttl time.Duration) error {
	if err := m.dropOldPartitions(ctx, "logs_restored", ttl); err != nil {
		return err
	}
	if err := m.dropOldPartitions(ctx, "traces_restored", ttl); err != nil {
		return err
	}
	if err := m.dropOldPartitions(ctx, "metrics_restored", ttl); err != nil {
		return err
	}
	return nil
}

func (m *PartitionManager) dropOldPartitions(ctx context.Context, parentTable string, retention time.Duration) error {
	if retention <= 0 {
		return nil
	}

	cutoff := truncateUTCDate(time.Now().UTC().Add(-retention))
	partitions, err := m.listDailyPartitions(ctx, parentTable)
	if err != nil {
		return fmt.Errorf("list partitions for %s: %w", parentTable, err)
	}

	for _, partition := range partitions {
		day, ok := partitionDate(partition)
		if !ok || !day.Before(cutoff) {
			continue
		}

		dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s", partition)
		if _, err := m.pool.Exec(ctx, dropQuery); err != nil {
			return fmt.Errorf("drop old partition %s: %w", partition, err)
		}

		m.logger.Info("dropped old partition",
			slog.String("table", parentTable),
			slog.String("partition", partition),
			slog.Time("day", day),
			slog.Time("cutoff", cutoff),
		)
	}

	return nil
}

func (m *PartitionManager) listDailyPartitions(ctx context.Context, parentTable string) ([]string, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT child.relname
		FROM pg_inherits
		JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
		JOIN pg_class child ON pg_inherits.inhrelid = child.oid
		WHERE parent.relname = $1
		  AND child.relname LIKE $2
		ORDER BY child.relname ASC
	`, parentTable, parentTable+"_p%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	partitions := make([]string, 0)
	for rows.Next() {
		var partition string
		if err := rows.Scan(&partition); err != nil {
			return nil, err
		}
		partitions = append(partitions, partition)
	}

	return partitions, rows.Err()
}

func partitionDate(partition string) (time.Time, bool) {
	parts := strings.Split(partition, "_p")
	if len(parts) < 2 {
		return time.Time{}, false
	}
	suffix := parts[len(parts)-1]
	day, err := time.Parse("20060102", suffix)
	if err != nil {
		return time.Time{}, false
	}
	return truncateUTCDate(day), true
}

func truncateUTCDate(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func sqlTimestampLiteral(value time.Time) string {
	return "'" + value.UTC().Format("2006-01-02 15:04:05-07:00") + "'"
}
