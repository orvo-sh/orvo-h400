package ingestservice

import (
	"encoding/hex"
	"math"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/otelutil"
	"github.com/orvo-sh/orvo/pkg/util"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

func (s *service) transformMetrics(resourceMetrics []*metricspb.ResourceMetrics, orgID string) []models.MetricPoint {
	var points []models.MetricPoint

	for _, rm := range resourceMetrics {
		resource := rm.GetResource()
		resourceAttrs := otelutil.KvListToMap(resource.GetAttributes())

		serviceName := resourceAttrs["service.name"]
		deploymentEnv := resourceAttrs["deployment.environment"]

		for _, sm := range rm.GetScopeMetrics() {
			scope := sm.GetScope()
			scopeName := scope.GetName()
			scopeVersion := scope.GetVersion()

			for _, metric := range sm.GetMetrics() {
				name := metric.GetName()
				unit := metric.GetUnit()
				description := metric.GetDescription()

				switch data := metric.GetData().(type) {
				case *metricspb.Metric_Sum:
					pts := s.transformSum(data.Sum, orgID, name, unit, description,
						serviceName, deploymentEnv, resourceAttrs, scopeName, scopeVersion)
					points = append(points, pts...)

				case *metricspb.Metric_Gauge:
					pts := s.transformGauge(data.Gauge, orgID, name, unit, description,
						serviceName, deploymentEnv, resourceAttrs, scopeName, scopeVersion)
					points = append(points, pts...)

				case *metricspb.Metric_Histogram:
					pts := s.transformHistogram(data.Histogram, orgID, name, unit, description,
						serviceName, deploymentEnv, resourceAttrs, scopeName, scopeVersion)
					points = append(points, pts...)

				case *metricspb.Metric_ExponentialHistogram:
					pts := s.transformExponentialHistogram(data.ExponentialHistogram, orgID, name, unit, description,
						serviceName, deploymentEnv, resourceAttrs, scopeName, scopeVersion)
					points = append(points, pts...)

				case *metricspb.Metric_Summary:
					// Summary is not supported; skip with a debug log.
					s.logger.Debug("transformMetrics: skipping unsupported Summary metric",
						"metric_name", name,
					)
				}
			}
		}
	}

	return points
}

func (s *service) transformSum(
	sum *metricspb.Sum,
	orgID, name, unit, description, serviceName, deploymentEnv string,
	resourceAttrs map[string]string, scopeName, scopeVersion string,
) []models.MetricPoint {
	var points []models.MetricPoint

	temporality := mapAggregationTemporality(sum.GetAggregationTemporality())
	isMonotonic := sum.GetIsMonotonic()

	for _, dp := range sum.GetDataPoints() {
		point := models.MetricPoint{
			OrganizationID:         orgID,
			MetricName:             name,
			MetricType:             models.MetricTypeSum,
			MetricUnit:             unit,
			Description:            description,
			ServiceName:            serviceName,
			DeploymentEnv:          deploymentEnv,
			ResourceAttrs:          resourceAttrs,
			ScopeName:              scopeName,
			ScopeVersion:           scopeVersion,
			Attributes:             otelutil.KvListToMap(dp.GetAttributes()),
			StartTime:              util.NanoToTime(dp.GetStartTimeUnixNano()),
			Time:                   util.NanoToTime(dp.GetTimeUnixNano()),
			AggregationTemporality: temporality,
			IsMonotonic:            isMonotonic,
			Flags:                  dp.GetFlags(),
		}

		// Set the numeric value.
		switch v := dp.GetValue().(type) {
		case *metricspb.NumberDataPoint_AsInt:
			val := v.AsInt
			point.ValueInt = &val
		case *metricspb.NumberDataPoint_AsDouble:
			val := v.AsDouble
			point.ValueDouble = &val
		}

		// Ensure time is set.
		if point.Time.IsZero() {
			point.Time = time.Now().UTC()
		}

		point.Exemplars = transformExemplars(dp.GetExemplars())
		points = append(points, point)
	}

	return points
}

func (s *service) transformGauge(
	gauge *metricspb.Gauge,
	orgID, name, unit, description, serviceName, deploymentEnv string,
	resourceAttrs map[string]string, scopeName, scopeVersion string,
) []models.MetricPoint {
	var points []models.MetricPoint

	for _, dp := range gauge.GetDataPoints() {
		point := models.MetricPoint{
			OrganizationID: orgID,
			MetricName:     name,
			MetricType:     models.MetricTypeGauge,
			MetricUnit:     unit,
			Description:    description,
			ServiceName:    serviceName,
			DeploymentEnv:  deploymentEnv,
			ResourceAttrs:  resourceAttrs,
			ScopeName:      scopeName,
			ScopeVersion:   scopeVersion,
			Attributes:     otelutil.KvListToMap(dp.GetAttributes()),
			StartTime:      util.NanoToTime(dp.GetStartTimeUnixNano()),
			Time:           util.NanoToTime(dp.GetTimeUnixNano()),
			Flags:          dp.GetFlags(),
		}

		switch v := dp.GetValue().(type) {
		case *metricspb.NumberDataPoint_AsInt:
			val := v.AsInt
			point.ValueInt = &val
		case *metricspb.NumberDataPoint_AsDouble:
			val := v.AsDouble
			point.ValueDouble = &val
		}

		if point.Time.IsZero() {
			point.Time = time.Now().UTC()
		}

		point.Exemplars = transformExemplars(dp.GetExemplars())
		points = append(points, point)
	}

	return points
}

func (s *service) transformHistogram(
	histogram *metricspb.Histogram,
	orgID, name, unit, description, serviceName, deploymentEnv string,
	resourceAttrs map[string]string, scopeName, scopeVersion string,
) []models.MetricPoint {
	var points []models.MetricPoint

	temporality := mapAggregationTemporality(histogram.GetAggregationTemporality())

	for _, dp := range histogram.GetDataPoints() {
		count := dp.GetCount()
		sum := dp.GetSum()
		min := dp.GetMin()
		max := dp.GetMax()

		point := models.MetricPoint{
			OrganizationID:          orgID,
			MetricName:              name,
			MetricType:              models.MetricTypeHistogram,
			MetricUnit:              unit,
			Description:             description,
			ServiceName:             serviceName,
			DeploymentEnv:           deploymentEnv,
			ResourceAttrs:           resourceAttrs,
			ScopeName:               scopeName,
			ScopeVersion:            scopeVersion,
			Attributes:              otelutil.KvListToMap(dp.GetAttributes()),
			StartTime:               util.NanoToTime(dp.GetStartTimeUnixNano()),
			Time:                    util.NanoToTime(dp.GetTimeUnixNano()),
			AggregationTemporality:  temporality,
			HistogramCount:          &count,
			HistogramSum:            &sum,
			HistogramMin:            &min,
			HistogramMax:            &max,
			HistogramBucketCounts:   dp.GetBucketCounts(),
			HistogramExplicitBounds: dp.GetExplicitBounds(),
			Flags:                   dp.GetFlags(),
		}

		if point.Time.IsZero() {
			point.Time = time.Now().UTC()
		}

		point.Exemplars = transformExemplars(dp.GetExemplars())
		points = append(points, point)
	}

	return points
}

// transformExponentialHistogram converts ExponentialHistogram data points into
// regular Histogram data points with explicit bucket boundaries.
func (s *service) transformExponentialHistogram(
	expHist *metricspb.ExponentialHistogram,
	orgID, name, unit, description, serviceName, deploymentEnv string,
	resourceAttrs map[string]string, scopeName, scopeVersion string,
) []models.MetricPoint {
	var points []models.MetricPoint

	temporality := mapAggregationTemporality(expHist.GetAggregationTemporality())

	for _, dp := range expHist.GetDataPoints() {
		count := dp.GetCount()
		sum := dp.GetSum()
		min := dp.GetMin()
		max := dp.GetMax()
		scale := int(dp.GetScale())

		// Convert exponential buckets to explicit boundaries and counts.
		explicitBounds, bucketCounts := convertExponentialBuckets(scale, dp.GetPositive(), dp.GetNegative(), dp.GetZeroCount())

		point := models.MetricPoint{
			OrganizationID:          orgID,
			MetricName:              name,
			MetricType:              models.MetricTypeHistogram,
			MetricUnit:              unit,
			Description:             description,
			ServiceName:             serviceName,
			DeploymentEnv:           deploymentEnv,
			ResourceAttrs:           resourceAttrs,
			ScopeName:               scopeName,
			ScopeVersion:            scopeVersion,
			Attributes:              otelutil.KvListToMap(dp.GetAttributes()),
			StartTime:               util.NanoToTime(dp.GetStartTimeUnixNano()),
			Time:                    util.NanoToTime(dp.GetTimeUnixNano()),
			AggregationTemporality:  temporality,
			HistogramCount:          &count,
			HistogramSum:            &sum,
			HistogramMin:            &min,
			HistogramMax:            &max,
			HistogramBucketCounts:   bucketCounts,
			HistogramExplicitBounds: explicitBounds,
			Flags:                   dp.GetFlags(),
		}

		if point.Time.IsZero() {
			point.Time = time.Now().UTC()
		}

		point.Exemplars = transformExponentialExemplars(dp.GetExemplars())
		points = append(points, point)
	}

	return points
}

// convertExponentialBuckets converts exponential histogram buckets into explicit boundaries.
// The exponential base is 2^(2^(-scale)), and bucket index `i` covers (base^i, base^(i+1)].
func convertExponentialBuckets(
	scale int,
	positive *metricspb.ExponentialHistogramDataPoint_Buckets,
	negative *metricspb.ExponentialHistogramDataPoint_Buckets,
	zeroCount uint64,
) ([]float64, []uint64) {
	base := math.Pow(2, math.Pow(2, float64(-scale)))

	type bucketEntry struct {
		lowerBound float64
		count      uint64
	}

	var entries []bucketEntry

	// Process negative buckets (mirrored: absolute values mapped via same scale).
	if negative != nil && len(negative.GetBucketCounts()) > 0 {
		offset := int(negative.GetOffset())
		for i, c := range negative.GetBucketCounts() {
			if c == 0 {
				continue
			}
			idx := offset + i
			// Negative bucket boundary: -(base^(idx+1))
			upperBound := -math.Pow(base, float64(idx))
			entries = append(entries, bucketEntry{lowerBound: upperBound, count: c})
		}
	}

	// Zero bucket.
	if zeroCount > 0 {
		entries = append(entries, bucketEntry{lowerBound: 0, count: zeroCount})
	}

	// Process positive buckets.
	if positive != nil && len(positive.GetBucketCounts()) > 0 {
		offset := int(positive.GetOffset())
		for i, c := range positive.GetBucketCounts() {
			if c == 0 {
				continue
			}
			idx := offset + i
			lowerBound := math.Pow(base, float64(idx))
			entries = append(entries, bucketEntry{lowerBound: lowerBound, count: c})
		}
	}

	if len(entries) == 0 {
		return nil, nil
	}

	// Build explicit bounds and bucket counts.
	// Explicit histogram has N boundaries and N+1 bucket counts.
	bounds := make([]float64, len(entries))
	counts := make([]uint64, len(entries)+1)

	for i, e := range entries {
		bounds[i] = e.lowerBound
		// Each entry's count goes into bucket i+1 (bucket 0 is for values <= bounds[0]).
		counts[i+1] = e.count
	}

	return bounds, counts
}

func transformExemplars(exemplars []*metricspb.Exemplar) []models.MetricExemplar {
	if len(exemplars) == 0 {
		return nil
	}

	result := make([]models.MetricExemplar, len(exemplars))
	for i, e := range exemplars {
		var value float64
		switch v := e.GetValue().(type) {
		case *metricspb.Exemplar_AsInt:
			value = float64(v.AsInt)
		case *metricspb.Exemplar_AsDouble:
			value = v.AsDouble
		}

		result[i] = models.MetricExemplar{
			TraceID:   hex.EncodeToString(e.GetTraceId()),
			SpanID:    hex.EncodeToString(e.GetSpanId()),
			Value:     value,
			Timestamp: util.NanoToTime(e.GetTimeUnixNano()),
		}
	}

	return result
}

func transformExponentialExemplars(exemplars []*metricspb.Exemplar) []models.MetricExemplar {
	// ExponentialHistogram uses the same Exemplar proto type.
	return transformExemplars(exemplars)
}

func mapAggregationTemporality(t metricspb.AggregationTemporality) models.AggregationTemporality {
	switch t {
	case metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA:
		return models.AggTemporalityDelta
	case metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE:
		return models.AggTemporalityCumulative
	default:
		return models.AggTemporalityUnspecified
	}
}
