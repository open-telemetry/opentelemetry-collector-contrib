// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter/internal"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type baseMetricSignal struct {
	ResourceSchemaURL           string              `json:"resource_schema_url"`
	ResourceAttributes          map[string]string   `json:"resource_attributes"`
	ServiceName                 string              `json:"service_name"`
	ScopeName                   string              `json:"scope_name"`
	ScopeVersion                string              `json:"scope_version"`
	ScopeSchemaURL              string              `json:"scope_schema_url"`
	ScopeAttributes             map[string]string   `json:"scope_attributes"`
	MetricName                  string              `json:"metric_name"`
	MetricDescription           string              `json:"metric_description"`
	MetricUnit                  string              `json:"metric_unit"`
	MetricAttributes            map[string]string   `json:"metric_attributes"`
	StartTimestamp              string              `json:"start_timestamp"`
	Timestamp                   string              `json:"timestamp"`
	Flags                       uint32              `json:"flags"`
	ExemplarsFilteredAttributes []map[string]string `json:"exemplars_filtered_attributes"`
	ExemplarsTimestamp          []string            `json:"exemplars_timestamp"`
	ExemplarsValue              []float64           `json:"exemplars_value"`
	ExemplarsSpanID             []string            `json:"exemplars_span_id"`
	ExemplarsTraceID            []string            `json:"exemplars_trace_id"`
}

type sumMetricSignal struct {
	baseMetricSignal
	Value                  float64 `json:"value"`
	AggregationTemporality int32   `json:"aggregation_temporality"`
	IsMonotonic            bool    `json:"is_monotonic"`
}

type gaugeMetricSignal struct {
	baseMetricSignal
	Value float64 `json:"value"`
}

type histogramMetricSignal struct {
	baseMetricSignal
	Count                  uint64    `json:"count"`
	Sum                    float64   `json:"sum"`
	BucketCounts           []uint64  `json:"bucket_counts"`
	ExplicitBounds         []float64 `json:"explicit_bounds"`
	Min                    *float64  `json:"min,omitempty"`
	Max                    *float64  `json:"max,omitempty"`
	AggregationTemporality int32     `json:"aggregation_temporality"`
}

type exponentialHistogramMetricSignal struct {
	baseMetricSignal
	Count                  uint64   `json:"count"`
	Sum                    float64  `json:"sum"`
	Scale                  int32    `json:"scale"`
	ZeroCount              uint64   `json:"zero_count"`
	PositiveOffset         int32    `json:"positive_offset"`
	PositiveBucketCounts   []uint64 `json:"positive_bucket_counts"`
	NegativeOffset         int32    `json:"negative_offset"`
	NegativeBucketCounts   []uint64 `json:"negative_bucket_counts"`
	Min                    *float64 `json:"min,omitempty"`
	Max                    *float64 `json:"max,omitempty"`
	AggregationTemporality int32    `json:"aggregation_temporality"`
}

func convertExemplars(exemplars pmetric.ExemplarSlice) ([]map[string]string, []string, []float64, []string, []string) {
	filteredAttributes := make([]map[string]string, exemplars.Len())
	timestamps := make([]string, exemplars.Len())
	values := make([]float64, exemplars.Len())
	spanIDs := make([]string, exemplars.Len())
	traceIDs := make([]string, exemplars.Len())
	for i := 0; i < exemplars.Len(); i++ {
		ex := exemplars.At(i)
		filteredAttributes[i] = convertAttributes(ex.FilteredAttributes())
		timestamps[i] = ex.Timestamp().AsTime().Format(time.RFC3339Nano)
		var value float64
		switch ex.ValueType() {
		case pmetric.ExemplarValueTypeInt:
			value = float64(ex.IntValue())
		case pmetric.ExemplarValueTypeDouble:
			value = ex.DoubleValue()
		case pmetric.ExemplarValueTypeEmpty:
			// Value is unset, use 0.0 as default
			value = 0.0
		}
		values[i] = value
		spanIDs[i] = traceutil.SpanIDToHexOrEmptyString(ex.SpanID())
		traceIDs[i] = traceutil.TraceIDToHexOrEmptyString(ex.TraceID())
	}
	return filteredAttributes, timestamps, values, spanIDs, traceIDs
}

func covertValue(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeEmpty:
		return 0.0
	}
	return 0.0
}

func ConvertMetrics(md pmetric.Metrics, sumEncoder, gaugeEncoder, histogramEncoder, exponentialHistogramEncoder Encoder) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resource := rm.Resource()
		schemaURL := rm.SchemaUrl()
		resourceAttributesMap := resource.Attributes()
		resourceAttributes := convertAttributes(resourceAttributesMap)
		serviceName := getServiceName(resourceAttributesMap)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scopeSchemaURL := sm.SchemaUrl()
			scope := sm.Scope()
			scopeName := scope.Name()
			scopeVersion := scope.Version()
			scopeAttributes := convertAttributes(scope.Attributes())
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)

				switch metric.Type() {
				case pmetric.MetricTypeSum:
					sum := metric.Sum()
					dps := sum.DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)

						filteredAttrs, timestamps, values, spanIDs, traceIDs := convertExemplars(dp.Exemplars())

						sumSignal := sumMetricSignal{
							baseMetricSignal: baseMetricSignal{
								ResourceSchemaURL:           schemaURL,
								ResourceAttributes:          resourceAttributes,
								ServiceName:                 serviceName,
								ScopeName:                   scopeName,
								ScopeVersion:                scopeVersion,
								ScopeSchemaURL:              scopeSchemaURL,
								ScopeAttributes:             scopeAttributes,
								MetricName:                  metric.Name(),
								MetricDescription:           metric.Description(),
								MetricUnit:                  metric.Unit(),
								MetricAttributes:            convertAttributes(dp.Attributes()),
								StartTimestamp:              dp.StartTimestamp().AsTime().Format(time.RFC3339Nano),
								Timestamp:                   dp.Timestamp().AsTime().Format(time.RFC3339Nano),
								Flags:                       uint32(dp.Flags()),
								ExemplarsFilteredAttributes: filteredAttrs,
								ExemplarsTimestamp:          timestamps,
								ExemplarsValue:              values,
								ExemplarsSpanID:             spanIDs,
								ExemplarsTraceID:            traceIDs,
							},
							Value:                  covertValue(dp),
							AggregationTemporality: int32(sum.AggregationTemporality()),
							IsMonotonic:            sum.IsMonotonic(),
						}
						if err := sumEncoder.Encode(sumSignal); err != nil {
							return err
						}
					}
				case pmetric.MetricTypeGauge:
					dps := metric.Gauge().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)

						filteredAttrs, timestamps, values, spanIDs, traceIDs := convertExemplars(dp.Exemplars())
						gaugeSignal := gaugeMetricSignal{
							baseMetricSignal: baseMetricSignal{
								ResourceSchemaURL:           schemaURL,
								ResourceAttributes:          resourceAttributes,
								ServiceName:                 serviceName,
								ScopeName:                   scopeName,
								ScopeVersion:                scopeVersion,
								ScopeSchemaURL:              scopeSchemaURL,
								ScopeAttributes:             scopeAttributes,
								MetricName:                  metric.Name(),
								MetricDescription:           metric.Description(),
								MetricUnit:                  metric.Unit(),
								MetricAttributes:            convertAttributes(dp.Attributes()),
								StartTimestamp:              dp.StartTimestamp().AsTime().Format(time.RFC3339Nano),
								Timestamp:                   dp.Timestamp().AsTime().Format(time.RFC3339Nano),
								Flags:                       uint32(dp.Flags()),
								ExemplarsFilteredAttributes: filteredAttrs,
								ExemplarsTimestamp:          timestamps,
								ExemplarsValue:              values,
								ExemplarsSpanID:             spanIDs,
								ExemplarsTraceID:            traceIDs,
							},
							Value: covertValue(dp),
						}
						if err := gaugeEncoder.Encode(gaugeSignal); err != nil {
							return err
						}
					}
				case pmetric.MetricTypeHistogram:
					hist := metric.Histogram()
					dps := hist.DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)

						var minVal, maxVal *float64
						if dp.HasMin() {
							localMin := dp.Min()
							minVal = &localMin
						}
						if dp.HasMax() {
							localMax := dp.Max()
							maxVal = &localMax
						}
						filteredAttrs, timestamps, values, spanIDs, traceIDs := convertExemplars(dp.Exemplars())
						histogramSignal := histogramMetricSignal{
							baseMetricSignal: baseMetricSignal{
								ResourceSchemaURL:           schemaURL,
								ResourceAttributes:          resourceAttributes,
								ServiceName:                 serviceName,
								ScopeName:                   scopeName,
								ScopeVersion:                scopeVersion,
								ScopeSchemaURL:              scopeSchemaURL,
								ScopeAttributes:             scopeAttributes,
								MetricName:                  metric.Name(),
								MetricDescription:           metric.Description(),
								MetricUnit:                  metric.Unit(),
								MetricAttributes:            convertAttributes(dp.Attributes()),
								StartTimestamp:              dp.StartTimestamp().AsTime().Format(time.RFC3339Nano),
								Timestamp:                   dp.Timestamp().AsTime().Format(time.RFC3339Nano),
								Flags:                       uint32(dp.Flags()),
								ExemplarsFilteredAttributes: filteredAttrs,
								ExemplarsTimestamp:          timestamps,
								ExemplarsValue:              values,
								ExemplarsSpanID:             spanIDs,
								ExemplarsTraceID:            traceIDs,
							},
							Count:                  dp.Count(),
							Sum:                    dp.Sum(),
							BucketCounts:           dp.BucketCounts().AsRaw(),
							ExplicitBounds:         dp.ExplicitBounds().AsRaw(),
							Min:                    minVal,
							Max:                    maxVal,
							AggregationTemporality: int32(hist.AggregationTemporality()),
						}
						if err := histogramEncoder.Encode(histogramSignal); err != nil {
							return err
						}
					}
				case pmetric.MetricTypeExponentialHistogram:
					ehist := metric.ExponentialHistogram()
					dps := ehist.DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)

						var minVal, maxVal *float64
						if dp.HasMin() {
							localMin := dp.Min()
							minVal = &localMin
						}
						if dp.HasMax() {
							localMax := dp.Max()
							maxVal = &localMax
						}
						filteredAttrs, timestamps, values, spanIDs, traceIDs := convertExemplars(dp.Exemplars())
						exponentialHistogramSignal := exponentialHistogramMetricSignal{
							baseMetricSignal: baseMetricSignal{
								ResourceSchemaURL:           schemaURL,
								ResourceAttributes:          resourceAttributes,
								ServiceName:                 serviceName,
								ScopeName:                   scopeName,
								ScopeVersion:                scopeVersion,
								ScopeSchemaURL:              scopeSchemaURL,
								ScopeAttributes:             scopeAttributes,
								MetricName:                  metric.Name(),
								MetricDescription:           metric.Description(),
								MetricUnit:                  metric.Unit(),
								MetricAttributes:            convertAttributes(dp.Attributes()),
								StartTimestamp:              dp.StartTimestamp().AsTime().Format(time.RFC3339Nano),
								Timestamp:                   dp.Timestamp().AsTime().Format(time.RFC3339Nano),
								Flags:                       uint32(dp.Flags()),
								ExemplarsFilteredAttributes: filteredAttrs,
								ExemplarsTimestamp:          timestamps,
								ExemplarsValue:              values,
								ExemplarsSpanID:             spanIDs,
								ExemplarsTraceID:            traceIDs,
							},
							Count:                  dp.Count(),
							Sum:                    dp.Sum(),
							Scale:                  dp.Scale(),
							ZeroCount:              dp.ZeroCount(),
							PositiveOffset:         dp.Positive().Offset(),
							PositiveBucketCounts:   dp.Positive().BucketCounts().AsRaw(),
							NegativeOffset:         dp.Negative().Offset(),
							NegativeBucketCounts:   dp.Negative().BucketCounts().AsRaw(),
							Min:                    minVal,
							Max:                    maxVal,
							AggregationTemporality: int32(ehist.AggregationTemporality()),
						}
						if err := exponentialHistogramEncoder.Encode(exponentialHistogramSignal); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}
