// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"context"
	"fmt"
	"maps"
	"math"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

/*
A converter package that converts and marshals data to be written to ADX metrics tables
*/
const (
	hostkey = "host.name"
	// Indicates the sum that is used in both summary and in histogram
	sumsuffix = "sum"
	// Count used in summary , histogram and also in exponential histogram
	countsuffix = "count"
	// Indicates the sum that is used in both summary and in histogram
	sumdescription = "(Sum total of samples)"
	// Count used in summary , histogram and also in exponential histogram
	countdescription = "(Count of samples)"
)

// This is derived from the specification https://opentelemetry.io/docs/reference/specification/metrics/datamodel/
type adxMetric struct {
	Timestamp string // The timestamp of the occurrence. A metric is measured at a point of time. Formatted into string as RFC3339Nano
	// Including name, the Metric object is defined by the following properties:
	MetricName        string         // Name of the metric field
	MetricType        string         // The data point type (e.g. Sum, Gauge, Histogram ExponentialHistogram, Summary)
	MetricUnit        string         // The metric stream’s unit
	MetricDescription string         // The metric stream’s description
	MetricValue       float64        // the value of the metric
	MetricAttributes  map[string]any // JSON attributes that can then be parsed. Extrinsic properties
	// Additional properties
	Host               string         // The hostname for analysis of the metric. Extracted from https://opentelemetry.io/docs/reference/specification/resource/semantic_conventions/host/
	ResourceAttributes map[string]any // The originating Resource attributes. Refer https://opentelemetry.io/docs/reference/specification/resource/sdk/
	// Exemplars are sampled measurements that carry the trace context (trace_id/span_id)
	// of an example request, bridging aggregate metrics to concrete traces. Only populated
	// when include_exemplars is enabled. Omitted from the payload when empty for backward
	// compatibility with the existing metrics table schema.
	Exemplars []map[string]any `json:"Exemplars,omitempty"` // [{value, timestamp, trace_id, span_id, attributes}]
}

/*
	Convert the pMetric to the type ADXMetric , this matches the scheme in the OTELMetric table in the database
*/

// jsonSafeFloat converts a float64 into a value that can be JSON-serialized. NaN and
// ±Inf have no JSON representation and make the encoder fail the entire row, so they
// are emitted in their string form instead.
func jsonSafeFloat(value float64) any {
	switch {
	case math.IsNaN(value):
		return "NaN"
	case math.IsInf(value, 1):
		return "Infinity"
	case math.IsInf(value, -1):
		return "-Infinity"
	default:
		return value
	}
}

// jsonSafeValue walks a raw attribute value and replaces any non-finite float so that a
// single exemplar attribute cannot fail serialization of the whole metric row.
func jsonSafeValue(value any) any {
	switch v := value.(type) {
	case float64:
		return jsonSafeFloat(v)
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = jsonSafeValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = jsonSafeValue(item)
		}
		return out
	default:
		return value
	}
}

// extractExemplars flattens an OTLP exemplar slice into a representation suitable for
// the ADX Exemplars dynamic column. trace_id/span_id use the same hex formatting as the
// traces exporter so they join directly against the OTELTraces table.
func extractExemplars(exemplars pmetric.ExemplarSlice) []map[string]any {
	if exemplars.Len() == 0 {
		return nil
	}
	out := make([]map[string]any, 0, exemplars.Len())
	for i := 0; i < exemplars.Len(); i++ {
		e := exemplars.At(i)
		exemplar := map[string]any{
			"timestamp":  e.Timestamp().AsTime().Format(time.RFC3339Nano),
			"trace_id":   traceutil.TraceIDToHexOrEmptyString(e.TraceID()),
			"span_id":    traceutil.SpanIDToHexOrEmptyString(e.SpanID()),
			"attributes": jsonSafeValue(e.FilteredAttributes().AsRaw()),
		}
		//exhaustive:enforce
		switch e.ValueType() {
		case pmetric.ExemplarValueTypeInt:
			// Kept as int64: widening to float64 would silently lose precision beyond 2^53.
			exemplar["value"] = e.IntValue()
		case pmetric.ExemplarValueTypeDouble:
			exemplar["value"] = jsonSafeFloat(e.DoubleValue())
		case pmetric.ExemplarValueTypeEmpty:
			// No recorded value; omit the key rather than reporting a synthetic 0.
		}
		out = append(out, exemplar)
	}
	return out
}

func mapToAdxMetric(res pcommon.Resource, md pmetric.Metric, scopeattrs map[string]any, includeExemplars bool, logger *zap.Logger) []*adxMetric {
	logger.Debug("Entering processing of toAdxMetric function")
	// default to collectors host name. Ignore the error here. This should not cause the failure of the process
	host, err := os.Hostname()
	if err != nil {
		logger.Warn("Default collector hostname could not be retrieved", zap.Error(err))
	}
	resourceAttrs := res.Attributes().AsRaw()
	if h := resourceAttrs[hostkey]; h != nil {
		host = h.(string)
	}
	createMetric := func(times time.Time, attr pcommon.Map, value func() float64, name, desc string, mt pmetric.MetricType) *adxMetric {
		clonedScopedAttributes := maps.Clone(scopeattrs)
		maps.Copy(clonedScopedAttributes, attr.AsRaw())
		if isEmpty(name) {
			name = md.Name()
		}
		if isEmpty(desc) {
			desc = md.Description()
		}
		return &adxMetric{
			Timestamp:          times.Format(time.RFC3339Nano),
			MetricName:         name,
			MetricType:         mt.String(),
			MetricUnit:         md.Unit(),
			MetricDescription:  desc,
			MetricValue:        value(),
			MetricAttributes:   clonedScopedAttributes,
			Host:               host,
			ResourceAttributes: resourceAttrs,
		}
	}
	//exhaustive:enforce
	switch md.Type() {
	case pmetric.MetricTypeGauge:
		dataPoints := md.Gauge().DataPoints()
		adxMetrics := make([]*adxMetric, dataPoints.Len())
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			adxMetric := createMetric(dataPoint.Timestamp().AsTime(), dataPoint.Attributes(), func() float64 {
				var metricValue float64
				switch dataPoint.ValueType() {
				case pmetric.NumberDataPointValueTypeInt:
					metricValue = float64(dataPoint.IntValue())
				case pmetric.NumberDataPointValueTypeDouble:
					metricValue = dataPoint.DoubleValue()
				}
				return metricValue
			}, "", "", pmetric.MetricTypeGauge)
			if includeExemplars {
				adxMetric.Exemplars = extractExemplars(dataPoint.Exemplars())
			}
			adxMetrics[gi] = adxMetric
		}
		return adxMetrics
	case pmetric.MetricTypeHistogram:
		dataPoints := md.Histogram().DataPoints()
		var adxMetrics []*adxMetric
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			bounds := dataPoint.ExplicitBounds()
			counts := dataPoint.BucketCounts()
			// Exemplars belong to the histogram data point as a whole; attach them to the
			// representative _count row so they can be joined to traces.
			countMetric := createMetric(dataPoint.Timestamp().AsTime(), dataPoint.Attributes(), func() float64 {
				// Change int to float. The value is a float64 in the table
				return float64(dataPoint.Count())
			},
				fmt.Sprintf("%s_%s", md.Name(), countsuffix),
				fmt.Sprintf("%s%s", md.Description(), countdescription),
				pmetric.MetricTypeHistogram)
			if includeExemplars {
				countMetric.Exemplars = extractExemplars(dataPoint.Exemplars())
			}
			adxMetrics = append(adxMetrics,
				// first, add one event for sum, and one for count
				createMetric(dataPoint.Timestamp().AsTime(), dataPoint.Attributes(), dataPoint.Sum,
					fmt.Sprintf("%s_%s", md.Name(), sumsuffix),
					fmt.Sprintf("%s%s", md.Description(), sumdescription),
					pmetric.MetricTypeHistogram),
				countMetric)
			// Spec says counts is optional but if present it must have one more
			// element than the bounds array.
			if counts.Len() == 0 || counts.Len() != bounds.Len()+1 {
				continue
			}
			value := uint64(0)
			// now create buckets for each bound.
			for bi := 0; bi < bounds.Len(); bi++ {
				customMap := map[string]any{"le": float64ToDimValue(bounds.At(bi))}
				maps.Copy(customMap, dataPoint.Attributes().AsRaw())

				value += counts.At(bi)
				vMap := pcommon.NewMap()
				//nolint:errcheck
				vMap.FromRaw(customMap)
				adxMetrics = append(adxMetrics, createMetric(dataPoint.Timestamp().AsTime(), vMap, func() float64 {
					// Change int to float. The value is a float64 in the table
					return float64(value)
				},
					fmt.Sprintf("%s_bucket", md.Name()),
					"",
					pmetric.MetricTypeHistogram))
			}
			// add an upper bound for +Inf
			{
				// Add the LE field for the bucket's bound
				customMap := map[string]any{"le": float64ToDimValue(math.Inf(1))}
				maps.Copy(customMap, dataPoint.Attributes().AsRaw())

				vMap := pcommon.NewMap()
				//nolint:errcheck
				vMap.FromRaw(customMap)
				adxMetrics = append(adxMetrics, createMetric(dataPoint.Timestamp().AsTime(), vMap, func() float64 {
					// Change int to float. The value is a float64 in the table
					return float64(value + counts.At(counts.Len()-1))
				},
					fmt.Sprintf("%s_bucket", md.Name()),
					"",
					pmetric.MetricTypeHistogram))
			}
		}
		return adxMetrics
	case pmetric.MetricTypeSum:
		dataPoints := md.Sum().DataPoints()
		adxMetrics := make([]*adxMetric, dataPoints.Len())
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			adxMetric := createMetric(dataPoint.Timestamp().AsTime(), dataPoint.Attributes(), func() float64 {
				var metricValue float64
				switch dataPoint.ValueType() {
				case pmetric.NumberDataPointValueTypeInt:
					metricValue = float64(dataPoint.IntValue())
				case pmetric.NumberDataPointValueTypeDouble:
					metricValue = dataPoint.DoubleValue()
				}
				return metricValue
			}, "", "", pmetric.MetricTypeSum)
			if includeExemplars {
				adxMetric.Exemplars = extractExemplars(dataPoint.Exemplars())
			}
			adxMetrics[gi] = adxMetric
		}
		return adxMetrics
	case pmetric.MetricTypeSummary:
		dataPoints := md.Summary().DataPoints()
		var adxMetrics []*adxMetric
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			adxMetrics = append(adxMetrics,
				// first, add one event for sum, and one for count
				createMetric(dataPoint.Timestamp().AsTime(), dataPoint.Attributes(), dataPoint.Sum,
					fmt.Sprintf("%s_%s", md.Name(), sumsuffix),
					fmt.Sprintf("%s%s", md.Description(), sumdescription),
					pmetric.MetricTypeSummary),
				// counts
				createMetric(dataPoint.Timestamp().AsTime(),
					dataPoint.Attributes(),
					func() float64 {
						return float64(dataPoint.Count())
					},
					fmt.Sprintf("%s_%s", md.Name(), countsuffix),
					fmt.Sprintf("%s%s", md.Description(), countdescription),
					pmetric.MetricTypeSummary))
			// now create values for each quantile.
			for bi := 0; bi < dataPoint.QuantileValues().Len(); bi++ {
				dp := dataPoint.QuantileValues().At(bi)
				quantileName := fmt.Sprintf("%s_%s", md.Name(), strconv.FormatFloat(dp.Quantile(), 'f', -1, 64))
				metricQuantile := map[string]any{
					"qt":         float64ToDimValue(dp.Quantile()),
					quantileName: sanitizeFloat(dp.Value()).(float64),
				}
				customMap := metricQuantile
				maps.Copy(customMap, dataPoint.Attributes().AsRaw())
				vMap := pcommon.NewMap()
				//nolint:errcheck
				vMap.FromRaw(customMap)
				adxMetrics = append(adxMetrics, createMetric(dataPoint.Timestamp().AsTime(),
					vMap,
					dp.Value,
					quantileName,
					fmt.Sprintf("%s%s", md.Description(), countdescription),
					pmetric.MetricTypeSummary))
			}
		}
		return adxMetrics
	default:
		logger.Warn(
			"Unsupported metric type : ",
			zap.Any("metric", md))
		return nil
	}
}

// Given all the metrics , transform that to the representative structure
func rawMetricsToAdxMetrics(_ context.Context, metrics pmetric.Metrics, includeExemplars bool, logger *zap.Logger) []*adxMetric {
	var transformedAdxMetrics []*adxMetric
	resourceMetric := metrics.ResourceMetrics()
	for i := 0; i < resourceMetric.Len(); i++ {
		res := resourceMetric.At(i).Resource()
		scopeMetrics := resourceMetric.At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			scopeMetric := scopeMetrics.At(j)
			metrics := scopeMetric.Metrics()
			// get details of the scope from the scope metric
			scopeAttr := getScopeMap(scopeMetric.Scope())
			for k := 0; k < metrics.Len(); k++ {
				transformedAdxMetrics = append(transformedAdxMetrics, mapToAdxMetric(res, metrics.At(k), scopeAttr, includeExemplars, logger)...)
			}
		}
	}
	return transformedAdxMetrics
}

func float64ToDimValue(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func sanitizeFloat(value float64) any {
	if math.IsNaN(value) {
		return math.NaN()
	}
	if math.IsInf(value, 1) {
		return math.Inf(1)
	}
	if math.IsInf(value, -1) {
		return math.Inf(-1)
	}
	return value
}
