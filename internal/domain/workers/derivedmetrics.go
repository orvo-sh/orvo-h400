package workers

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/clickhouse"
	"github.com/orvo-sh/orvo/internal/sink"
)

// DerivedMetricsConfig holds configuration for the derived metrics worker.
type DerivedMetricsConfig struct {
	// ComputeInterval is how often the worker runs a computation cycle.
	// Default: 1 minute.
	ComputeInterval time.Duration

	// Lookback is how far back the worker looks for source data.
	// Default: 2 minutes (with overlap to avoid gaps).
	Lookback time.Duration

	// ApdexThreshold is the "satisfied" threshold in milliseconds for Apdex score.
	// Default: 500ms. Frustrated = 4 * ApdexThreshold.
	ApdexThreshold float64
}

func (c *DerivedMetricsConfig) defaults() {
	if c.ComputeInterval == 0 {
		c.ComputeInterval = 1 * time.Minute
	}
	if c.Lookback == 0 {
		c.Lookback = 2 * time.Minute
	}
	if c.ApdexThreshold == 0 {
		c.ApdexThreshold = 500.0 // 500ms
	}
}

// DerivedMetricsWorker periodically computes complex derived metrics
// (Apdex, health scores, error budgets, availability) that cannot be
// expressed as ClickHouse materialized views.
//
// It reads source data from ClickHouse (spans, logs, metrics) and writes
// computed metric points back through MetricSink so they flow into the
// standard rollup pipeline.
type DerivedMetricsWorker struct {
	ch         *clickhouse.DB
	metricSink *sink.MetricSink
	logger     *slog.Logger
	config     DerivedMetricsConfig

	done chan struct{}
	wg   sync.WaitGroup
	once sync.Once
}

// NewDerivedMetricsWorker creates a new derived metrics worker.
func NewDerivedMetricsWorker(
	ch *clickhouse.DB,
	metricSink *sink.MetricSink,
	logger *slog.Logger,
	config DerivedMetricsConfig,
) *DerivedMetricsWorker {
	config.defaults()
	return &DerivedMetricsWorker{
		ch:         ch,
		metricSink: metricSink,
		logger:     logger.With("worker", "derived_metrics"),
		config:     config,
		done:       make(chan struct{}),
	}
}

// Start begins the worker loop. Call Stop() to gracefully shut down.
func (w *DerivedMetricsWorker) Start() {
	w.wg.Add(1)
	go w.loop()
	w.logger.Info("derived metrics worker started",
		slog.Duration("interval", w.config.ComputeInterval),
		slog.Duration("lookback", w.config.Lookback),
		slog.Float64("apdex_threshold_ms", w.config.ApdexThreshold),
	)
}

// Stop signals the worker to stop and waits for it to finish.
func (w *DerivedMetricsWorker) Stop() {
	w.once.Do(func() {
		close(w.done)
		w.wg.Wait()
		w.logger.Info("derived metrics worker stopped")
	})
}

func (w *DerivedMetricsWorker) loop() {
	defer w.wg.Done()

	// Run immediately on start, then on ticker.
	w.runCycle()

	ticker := time.NewTicker(w.config.ComputeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.runCycle()
		case <-w.done:
			return
		}
	}
}

func (w *DerivedMetricsWorker) runCycle() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now().UTC()
	windowStart := now.Add(-w.config.Lookback)

	// Find all active organization+service pairs in the window.
	orgServices, err := w.getActiveOrgServices(ctx, windowStart, now)
	if err != nil {
		w.logger.Error("failed to get active org/service pairs", slog.Any("error", err))
		return
	}

	if len(orgServices) == 0 {
		return
	}

	var allPoints []models.MetricPoint

	for _, os := range orgServices {
		points, err := w.computeForOrgService(ctx, os.OrgID, os.Service, windowStart, now)
		if err != nil {
			w.logger.Error("failed to compute derived metrics",
				slog.String("org_id", os.OrgID),
				slog.String("service", os.Service),
				slog.Any("error", err),
			)
			continue
		}
		allPoints = append(allPoints, points...)
	}

	if len(allPoints) == 0 {
		return
	}

	if err := w.metricSink.Enqueue(ctx, allPoints); err != nil {
		w.logger.Error("failed to enqueue derived metric points",
			slog.Int("count", len(allPoints)),
			slog.Any("error", err),
		)
		return
	}

	w.logger.Debug("derived metrics cycle complete",
		slog.Int("org_services", len(orgServices)),
		slog.Int("points", len(allPoints)),
	)
}

type orgService struct {
	OrgID   string
	Service string
}

// getActiveOrgServices finds distinct organization_id + service_name pairs
// that have recent span data.
func (w *DerivedMetricsWorker) getActiveOrgServices(ctx context.Context, start, end time.Time) ([]orgService, error) {
	query := `SELECT DISTINCT organization_id, service_name
		FROM spans
		WHERE start_time >= ? AND start_time <= ?
		  AND service_name != ''
		ORDER BY organization_id, service_name`

	rows, err := w.ch.Query(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("query active org/services: %w", err)
	}
	defer rows.Close()

	var result []orgService
	for rows.Next() {
		var os orgService
		if err := rows.Scan(&os.OrgID, &os.Service); err != nil {
			return nil, fmt.Errorf("scan org/service: %w", err)
		}
		result = append(result, os)
	}
	return result, rows.Err()
}

// computeForOrgService computes all derived metrics for a single org+service pair.
func (w *DerivedMetricsWorker) computeForOrgService(
	ctx context.Context,
	orgID, serviceName string,
	windowStart, windowEnd time.Time,
) ([]models.MetricPoint, error) {
	now := windowEnd
	var points []models.MetricPoint

	// 1. Apdex score
	apdex, err := w.computeApdex(ctx, orgID, serviceName, windowStart, windowEnd)
	if err != nil {
		return nil, fmt.Errorf("compute apdex: %w", err)
	}
	points = append(points, newDerivedPoint(orgID, serviceName, "service.apdex", "",
		"Apdex score (0-1) measuring user satisfaction", apdex, now))

	// 2. Error rate percentage
	errorRate, err := w.computeErrorRate(ctx, orgID, serviceName, windowStart, windowEnd)
	if err != nil {
		return nil, fmt.Errorf("compute error rate: %w", err)
	}
	points = append(points, newDerivedPoint(orgID, serviceName, "service.error_rate", "%",
		"Percentage of requests resulting in errors", errorRate, now))

	// 3. Availability (1 - error_rate/100)
	availability := 100.0 - errorRate
	if availability < 0 {
		availability = 0
	}
	points = append(points, newDerivedPoint(orgID, serviceName, "service.availability", "%",
		"Service availability percentage", availability, now))

	// 4. Error budget remaining (assuming 99.9% SLO)
	sloTarget := 99.9
	errorBudgetTotal := 100.0 - sloTarget // 0.1%
	errorBudgetUsed := errorRate
	errorBudgetRemaining := 0.0
	if errorBudgetTotal > 0 {
		errorBudgetRemaining = math.Max(0, (1.0-(errorBudgetUsed/errorBudgetTotal))*100.0)
	}
	points = append(points, newDerivedPoint(orgID, serviceName, "service.error_budget_remaining", "%",
		"Percentage of error budget remaining (99.9% SLO)", errorBudgetRemaining, now))

	// 5. Health score (composite: 40% apdex + 30% availability + 30% error budget)
	healthScore := apdex*40.0 + (availability/100.0)*30.0 + (errorBudgetRemaining/100.0)*30.0
	points = append(points, newDerivedPoint(orgID, serviceName, "service.health_score", "",
		"Composite service health score (0-100)", healthScore, now))

	// 6. Log error rate (errors per minute from logs)
	logErrorRate, err := w.computeLogErrorRate(ctx, orgID, serviceName, windowStart, windowEnd)
	if err != nil {
		// Non-fatal: log services may not have log data.
		w.logger.Debug("skipping log error rate (no data or error)",
			slog.String("org_id", orgID),
			slog.String("service", serviceName),
			slog.Any("error", err),
		)
	} else {
		points = append(points, newDerivedPoint(orgID, serviceName, "service.log_error_rate", "1/min",
			"Error log records per minute", logErrorRate, now))
	}

	return points, nil
}

// computeApdex calculates the Apdex score.
// Apdex = (satisfied + tolerating/2) / total
// satisfied: duration <= threshold
// tolerating: threshold < duration <= 4*threshold
// frustrated: duration > 4*threshold
func (w *DerivedMetricsWorker) computeApdex(
	ctx context.Context,
	orgID, serviceName string,
	start, end time.Time,
) (float64, error) {
	threshold := w.config.ApdexThreshold
	frustratedThreshold := threshold * 4.0

	query := `SELECT
		countIf(duration_ns / 1000000.0 <= ?) AS satisfied,
		countIf(duration_ns / 1000000.0 > ? AND duration_ns / 1000000.0 <= ?) AS tolerating,
		count(*) AS total
	FROM spans
	WHERE organization_id = ?
	  AND service_name = ?
	  AND start_time >= ?
	  AND start_time <= ?`

	rows, err := w.ch.Query(ctx, query,
		threshold, threshold, frustratedThreshold,
		orgID, serviceName, start, end,
	)
	if err != nil {
		return 0, fmt.Errorf("apdex query: %w", err)
	}
	defer rows.Close()

	var satisfied, tolerating, total uint64
	if rows.Next() {
		if err := rows.Scan(&satisfied, &tolerating, &total); err != nil {
			return 0, fmt.Errorf("apdex scan: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if total == 0 {
		return 1.0, nil // No data = perfect score.
	}

	apdex := (float64(satisfied) + float64(tolerating)/2.0) / float64(total)
	return math.Round(apdex*1000) / 1000, nil // Round to 3 decimal places.
}

// computeErrorRate calculates the error percentage (errors / total * 100).
func (w *DerivedMetricsWorker) computeErrorRate(
	ctx context.Context,
	orgID, serviceName string,
	start, end time.Time,
) (float64, error) {
	query := `SELECT
		countIf(status_code = 2) AS errors,
		count(*) AS total
	FROM spans
	WHERE organization_id = ?
	  AND service_name = ?
	  AND start_time >= ?
	  AND start_time <= ?`

	rows, err := w.ch.Query(ctx, query, orgID, serviceName, start, end)
	if err != nil {
		return 0, fmt.Errorf("error rate query: %w", err)
	}
	defer rows.Close()

	var errors, total uint64
	if rows.Next() {
		if err := rows.Scan(&errors, &total); err != nil {
			return 0, fmt.Errorf("error rate scan: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if total == 0 {
		return 0, nil
	}

	rate := float64(errors) / float64(total) * 100.0
	return math.Round(rate*100) / 100, nil // Round to 2 decimal places.
}

// computeLogErrorRate computes the log error records per minute.
func (w *DerivedMetricsWorker) computeLogErrorRate(
	ctx context.Context,
	orgID, serviceName string,
	start, end time.Time,
) (float64, error) {
	query := `SELECT
		count(*) AS error_count
	FROM logs
	WHERE organization_id = ?
	  AND service_name = ?
	  AND timestamp >= ?
	  AND timestamp <= ?
	  AND severity_number >= 17`

	rows, err := w.ch.Query(ctx, query, orgID, serviceName, start, end)
	if err != nil {
		return 0, fmt.Errorf("log error rate query: %w", err)
	}
	defer rows.Close()

	var errorCount uint64
	if rows.Next() {
		if err := rows.Scan(&errorCount); err != nil {
			return 0, fmt.Errorf("log error rate scan: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	windowMinutes := end.Sub(start).Minutes()
	if windowMinutes <= 0 {
		windowMinutes = 1
	}

	return math.Round(float64(errorCount)/windowMinutes*100) / 100, nil
}

// newDerivedPoint creates a gauge MetricPoint for a derived metric.
func newDerivedPoint(
	orgID, serviceName, metricName, unit, description string,
	value float64, ts time.Time,
) models.MetricPoint {
	return models.MetricPoint{
		OrganizationID:         orgID,
		MetricName:             metricName,
		MetricType:             models.MetricTypeGauge,
		MetricUnit:             unit,
		Description:            description,
		ServiceName:            serviceName,
		ResourceAttrs:          map[string]string{},
		Attributes:             map[string]string{},
		StartTime:              ts,
		Time:                   ts,
		ValueDouble:            &value,
		AggregationTemporality: models.AggTemporalityUnspecified,
	}
}
