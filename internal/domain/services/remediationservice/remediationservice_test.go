package remediationservice

import (
	"testing"

	"github.com/orvo-sh/orvo/internal/domain/models"
)

func TestIsErrorLog(t *testing.T) {
	tests := []struct {
		name   string
		record *models.LogRecord
		want   bool
	}{
		{
			name: "error by severity number",
			record: &models.LogRecord{
				SeverityNumber: 17,
				SeverityText:   "INFO",
			},
			want: true,
		},
		{
			name: "error by severity text",
			record: &models.LogRecord{
				SeverityNumber: 9,
				SeverityText:   "error",
			},
			want: true,
		},
		{
			name: "non error",
			record: &models.LogRecord{
				SeverityNumber: 9,
				SeverityText:   "info",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isErrorLog(tt.record); got != tt.want {
				t.Fatalf("isErrorLog() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeForCommitScope(t *testing.T) {
	if got := normalizeForCommitScope("API Gateway"); got != "api-gateway" {
		t.Fatalf("normalizeForCommitScope() = %q, want %q", got, "api-gateway")
	}
	if got := normalizeForCommitScope("  "); got != "service" {
		t.Fatalf("normalizeForCommitScope() with blank input = %q, want %q", got, "service")
	}
}
