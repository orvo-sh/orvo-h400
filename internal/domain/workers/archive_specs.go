package workers

import (
	"fmt"
	"strings"
	"time"
)

type TelemetrySignal string

const (
	SignalLogs    TelemetrySignal = "logs"
	SignalTraces  TelemetrySignal = "traces"
	SignalMetrics TelemetrySignal = "metrics"
)

type signalSpec struct {
	Signal       TelemetrySignal
	HotTable     string
	RestoreTable string
	TimeColumn   string
	Columns      []string
}

var signalSpecs = map[TelemetrySignal]signalSpec{
	SignalLogs: {
		Signal:       SignalLogs,
		HotTable:     "logs_hot",
		RestoreTable: "logs_restored",
		TimeColumn:   `"timestamp"`,
		Columns: []string{
			"id",
			"organization_id",
			`"timestamp"`,
			"observed_timestamp",
			"severity_number",
			"severity_text",
			"body",
			"trace_id",
			"span_id",
			"trace_flags",
			"resource_attributes",
			"resource_schema_url",
			"scope_name",
			"scope_version",
			"scope_attributes",
			"scope_schema_url",
			"log_attributes",
			"service_name",
			"deployment_environment",
		},
	},
	SignalTraces: {
		Signal:       SignalTraces,
		HotTable:     "traces_hot",
		RestoreTable: "traces_restored",
		TimeColumn:   "start_time",
		Columns: []string{
			"id",
			"organization_id",
			"trace_id",
			"span_id",
			"parent_span_id",
			"trace_state",
			"name",
			"kind",
			"start_time",
			"end_time",
			"duration_ns",
			"status_code",
			"status_message",
			"resource_attributes",
			"scope_attributes",
			"span_attributes",
			"resource_schema_url",
			"scope_name",
			"scope_version",
			"scope_schema_url",
			"events",
			"links",
			"service_name",
			"deployment_environment",
		},
	},
	SignalMetrics: {
		Signal:       SignalMetrics,
		HotTable:     "metrics_hot",
		RestoreTable: "metrics_restored",
		TimeColumn:   `"time"`,
		Columns: []string{
			"id",
			"organization_id",
			"metric_name",
			"metric_type",
			"metric_unit",
			"metric_description",
			"service_name",
			"deployment_environment",
			"resource_attributes",
			"scope_name",
			"scope_version",
			"attributes",
			"start_time",
			`"time"`,
			"value_int",
			"value_double",
			"aggregation_temporality",
			"is_monotonic",
			"histogram_count",
			"histogram_sum",
			"histogram_min",
			"histogram_max",
			"histogram_bucket_counts",
			"histogram_explicit_bounds",
			"exemplars",
			"flags",
		},
	},
}

func parseSignal(value string) (TelemetrySignal, bool) {
	s := TelemetrySignal(strings.ToLower(strings.TrimSpace(value)))
	_, ok := signalSpecs[s]
	return s, ok
}

func ParseTelemetrySignal(value string) (TelemetrySignal, bool) {
	return parseSignal(value)
}

func getSignalSpec(signal TelemetrySignal) (signalSpec, bool) {
	spec, ok := signalSpecs[signal]
	return spec, ok
}

func datesBetween(start time.Time, end time.Time) []time.Time {
	startDay := truncateUTCDate(start)
	endDay := truncateUTCDate(end)
	if endDay.Before(startDay) {
		startDay, endDay = endDay, startDay
	}

	days := make([]time.Time, 0, int(endDay.Sub(startDay).Hours()/24)+1)
	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		days = append(days, day)
	}
	return days
}

func sqlStringLiteral(value string) string {
	escaped := strings.ReplaceAll(value, `'`, `''`)
	return fmt.Sprintf("'%s'", escaped)
}

func sqlDateLiteral(value time.Time) string {
	return sqlStringLiteral(truncateUTCDate(value).Format("2006-01-02"))
}
