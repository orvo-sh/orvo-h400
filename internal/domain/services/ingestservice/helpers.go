package ingestservice

import (
	"encoding/hex"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/otelutil"
	"github.com/orvo-sh/orvo/pkg/util"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	tracespb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func (s *service) transformLogs(resourceLogs []*logspb.ResourceLogs, orgID string) []models.LogRecord {
	var records []models.LogRecord

	for _, rl := range resourceLogs {
		resource := rl.GetResource()
		resourceAttrs := otelutil.KvListToMap(resource.GetAttributes())
		resourceSchemaURL := rl.GetSchemaUrl()

		serviceName := resourceAttrs["service.name"]
		deploymentEnv := resourceAttrs["deployment.environment"]

		for _, sl := range rl.GetScopeLogs() {
			scope := sl.GetScope()
			scopeName := scope.GetName()
			scopeVersion := scope.GetVersion()
			scopeAttrs := otelutil.KvListToMap(scope.GetAttributes())
			scopeSchemaURL := sl.GetSchemaUrl()

			for _, lr := range sl.GetLogRecords() {
				timestamp := util.NanoToTime(lr.GetTimeUnixNano())
				observedTimestamp := util.NanoToTime(lr.GetObservedTimeUnixNano())

				// If timestamp is zero, use observed timestamp.
				if timestamp.IsZero() {
					timestamp = observedTimestamp
				}
				// If observed timestamp is zero, use now.
				if observedTimestamp.IsZero() {
					observedTimestamp = time.Now().UTC()
				}

				records = append(records, models.LogRecord{
					ID:                    util.GenerateID("log"),
					Timestamp:             timestamp,
					ObservedTimestamp:     observedTimestamp,
					SeverityNumber:        uint8(lr.GetSeverityNumber()),
					SeverityText:          lr.GetSeverityText(),
					Body:                  otelutil.AnyValueToString(lr.GetBody()),
					TraceID:               hex.EncodeToString(lr.GetTraceId()),
					SpanID:                hex.EncodeToString(lr.GetSpanId()),
					TraceFlags:            lr.GetFlags(),
					ResourceAttributes:    resourceAttrs,
					ResourceSchemaURL:     resourceSchemaURL,
					ScopeName:             scopeName,
					ScopeVersion:          scopeVersion,
					ScopeAttributes:       scopeAttrs,
					ScopeSchemaURL:        scopeSchemaURL,
					LogAttributes:         otelutil.KvListToMap(lr.GetAttributes()),
					ServiceName:           serviceName,
					DeploymentEnvironment: deploymentEnv,
					OrganizationID:        orgID,
				})
			}
		}
	}

	return records
}

func (s *service) transformTraces(resourceSpans []*tracespb.ResourceSpans, orgID string) []models.Span {
	var spans []models.Span

	for _, rs := range resourceSpans {
		resource := rs.GetResource()
		resourceAttrs := otelutil.KvListToMap(resource.GetAttributes())
		resourceSchemaURL := rs.GetSchemaUrl()

		serviceName := resourceAttrs["service.name"]
		deploymentEnv := resourceAttrs["deployment.environment"]

		for _, ss := range rs.GetScopeSpans() {
			scope := ss.GetScope()
			scopeName := scope.GetName()
			scopeVersion := scope.GetVersion()
			scopeAttrs := otelutil.KvListToMap(scope.GetAttributes())
			scopeSchemaURL := ss.GetSchemaUrl()

			for _, sp := range ss.GetSpans() {
				startTime := util.NanoToTime(sp.GetStartTimeUnixNano())
				endTime := util.NanoToTime(sp.GetEndTimeUnixNano())

				// If start time is zero, use now.
				if startTime.IsZero() {
					startTime = time.Now().UTC()
				}
				// If end time is zero, use start time.
				if endTime.IsZero() {
					endTime = startTime
				}

				durationNs := endTime.Sub(startTime).Nanoseconds()

				// Convert events.
				protoEvents := sp.GetEvents()
				events := make([]models.SpanEvent, len(protoEvents))
				for i, e := range protoEvents {
					events[i] = models.SpanEvent{
						Name:       e.GetName(),
						Timestamp:  util.NanoToTime(e.GetTimeUnixNano()),
						Attributes: otelutil.KvListToMap(e.GetAttributes()),
					}
				}

				// Convert links.
				protoLinks := sp.GetLinks()
				links := make([]models.SpanLink, len(protoLinks))
				for i, l := range protoLinks {
					links[i] = models.SpanLink{
						TraceID:    hex.EncodeToString(l.GetTraceId()),
						SpanID:     hex.EncodeToString(l.GetSpanId()),
						TraceState: l.GetTraceState(),
						Attributes: otelutil.KvListToMap(l.GetAttributes()),
					}
				}

				// Determine status.
				status := sp.GetStatus()
				statusCode := uint8(0)
				statusMessage := ""
				if status != nil {
					statusCode = uint8(status.GetCode())
					statusMessage = status.GetMessage()
				}

				spans = append(spans, models.Span{
					ID:                    util.GenerateID("span"),
					OrganizationID:        orgID,
					TraceID:               hex.EncodeToString(sp.GetTraceId()),
					SpanID:                hex.EncodeToString(sp.GetSpanId()),
					ParentSpanID:          hex.EncodeToString(sp.GetParentSpanId()),
					TraceState:            sp.GetTraceState(),
					Name:                  sp.GetName(),
					Kind:                  uint8(sp.GetKind()),
					StartTime:             startTime,
					EndTime:               endTime,
					DurationNs:            durationNs,
					StatusCode:            statusCode,
					StatusMessage:         statusMessage,
					ResourceAttributes:    resourceAttrs,
					ScopeAttributes:       scopeAttrs,
					SpanAttributes:        otelutil.KvListToMap(sp.GetAttributes()),
					ResourceSchemaURL:     resourceSchemaURL,
					ScopeName:             scopeName,
					ScopeVersion:          scopeVersion,
					ScopeSchemaURL:        scopeSchemaURL,
					Events:                events,
					Links:                 links,
					ServiceName:           serviceName,
					DeploymentEnvironment: deploymentEnv,
				})
			}
		}
	}

	return spans
}
