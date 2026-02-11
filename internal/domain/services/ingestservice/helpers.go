package ingestservice

import (
	"encoding/hex"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/otelutil"
	"github.com/orvo-sh/orvo/pkg/util"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
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
