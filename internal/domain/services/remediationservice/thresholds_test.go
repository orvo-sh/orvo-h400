package remediationservice

import (
	"testing"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
)

func TestEvaluateThresholdSeries(t *testing.T) {
	points := []models.TimeseriesPoint{
		{Time: time.Unix(0, 0), Value: 0.02},
		{Time: time.Unix(60, 0), Value: 0.08},
	}

	evaluation := evaluateThresholdSeries(points, 0.05, 5, 60)
	if !evaluation.Breached {
		t.Fatalf("expected evaluation to breach")
	}
	if evaluation.EstimatedErrorCount != 6 {
		t.Fatalf("expected 6 estimated errors, got %d", evaluation.EstimatedErrorCount)
	}
}
