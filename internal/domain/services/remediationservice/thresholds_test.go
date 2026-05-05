package remediationservice

import (
	"testing"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
)

func TestEvaluateThresholdSeries(t *testing.T) {
	points := []models.TimeseriesPoint{
		{Time: time.Unix(0, 0), Value: 4},
		{Time: time.Unix(60, 0), Value: 6},
	}

	evaluation := evaluateThresholdSeries(points, 5, 5, 60)
	if !evaluation.Breached {
		t.Fatalf("expected evaluation to breach")
	}
	if evaluation.ObservedErrorCount != 10 {
		t.Fatalf("expected 10 observed errors, got %d", evaluation.ObservedErrorCount)
	}
}

func TestEvaluateThresholdSeriesDoesNotBreachBelowThreshold(t *testing.T) {
	points := []models.TimeseriesPoint{
		{Time: time.Unix(0, 0), Value: 2},
		{Time: time.Unix(60, 0), Value: 2},
	}

	evaluation := evaluateThresholdSeries(points, 5, 5, 60)
	if evaluation.Breached {
		t.Fatalf("expected evaluation not to breach")
	}
}
