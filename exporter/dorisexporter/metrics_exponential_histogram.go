// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	_ "embed"
	"fmt"
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

//go:embed sql/metrics_exponential_histogram_ddl.sql
var metricsExponentialHistogramDDL string

// dMetricExponentialHistogram Exponential Histogram Metric to Doris
type dMetricExponentialHistogram struct {
	*dMetric               `json:",inline"`
	Timestamp              string         `json:"timestamp"`
	Attributes             map[string]any `json:"attributes"`
	StartTime              string         `json:"start_time"`
	Count                  int64          `json:"count"`
	Sum                    float64        `json:"sum"`
	Scale                  int32          `json:"scale"`
	ZeroCount              int64          `json:"zero_count"`
	PositiveOffset         int32          `json:"positive_offset"`
	PositiveBucketCounts   []int64        `json:"positive_bucket_counts"`
	NegativeOffset         int32          `json:"negative_offset"`
	NegativeBucketCounts   []int64        `json:"negative_bucket_counts"`
	Exemplars              []*dExemplar   `json:"exemplars"`
	Min                    float64        `json:"min"`
	Max                    float64        `json:"max"`
	ZeroThreshold          float64        `json:"zero_threshold"`
	AggregationTemporality string         `json:"aggregation_temporality"`
}

type metricModelExponentialHistogram struct {
	metricModelCommon[dMetricExponentialHistogram]
}

func (*metricModelExponentialHistogram) metricType() pmetric.MetricType {
	return pmetric.MetricTypeExponentialHistogram
}

func (*metricModelExponentialHistogram) tableSuffix() string {
	return "_exponential_histogram"
}

func (m *metricModelExponentialHistogram) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter) error {
	if pm.Type() != pmetric.MetricTypeExponentialHistogram {
		return fmt.Errorf("metric type is not exponential histogram: %v", pm.Type().String())
	}

	dataPoints := pm.ExponentialHistogram().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		if dp.Flags().NoRecordedValue() {
			e.logger.Warn("dropping exponential histogram datapoint with NoRecordedValue flag", zap.String("metric_name", dm.MetricName))
			continue
		}

		if dp.HasSum() && (math.IsNaN(dp.Sum()) || math.IsInf(dp.Sum(), 0)) {
			e.logger.Warn("dropping exponential histogram datapoint with non-finite sum", zap.String("metric_name", dm.MetricName), zap.Float64("sum", dp.Sum()))
			continue
		}
		if dp.HasMin() && (math.IsNaN(dp.Min()) || math.IsInf(dp.Min(), 0)) {
			e.logger.Warn("dropping exponential histogram datapoint with non-finite min", zap.String("metric_name", dm.MetricName), zap.Float64("min", dp.Min()))
			continue
		}
		if dp.HasMax() && (math.IsNaN(dp.Max()) || math.IsInf(dp.Max(), 0)) {
			e.logger.Warn("dropping exponential histogram datapoint with non-finite max", zap.String("metric_name", dm.MetricName), zap.Float64("max", dp.Max()))
			continue
		}
		if math.IsNaN(dp.ZeroThreshold()) || math.IsInf(dp.ZeroThreshold(), 0) {
			e.logger.Warn("dropping exponential histogram datapoint with non-finite zero threshold", zap.String("metric_name", dm.MetricName), zap.Float64("zero_threshold", dp.ZeroThreshold()))
			continue
		}

		exemplars := dp.Exemplars()
		newExemplars := make([]*dExemplar, 0, exemplars.Len())
		for j := 0; j < exemplars.Len(); j++ {
			exemplar := exemplars.At(j)
			if exemplar.ValueType() == pmetric.ExemplarValueTypeDouble {
				v := exemplar.DoubleValue()
				if math.IsNaN(v) || math.IsInf(v, 0) {
					e.logger.Warn("dropping exemplar with non-finite value", zap.String("metric_name", dm.MetricName), zap.Float64("value", v))
					continue
				}
			}

			newExemplar := &dExemplar{
				FilteredAttributes: exemplar.FilteredAttributes().AsRaw(),
				Timestamp:          e.formatTime(exemplar.Timestamp().AsTime()),
				Value:              e.getExemplarValue(exemplar),
				SpanID:             exemplar.SpanID().String(),
				TraceID:            exemplar.TraceID().String(),
			}

			newExemplars = append(newExemplars, newExemplar)
		}

		positiveBucketCounts := dp.Positive().BucketCounts()
		newPositiveBucketCounts := make([]int64, 0, positiveBucketCounts.Len())
		for j := 0; j < positiveBucketCounts.Len(); j++ {
			newPositiveBucketCounts = append(newPositiveBucketCounts, int64(positiveBucketCounts.At(j)))
		}

		negativeBucketCounts := dp.Negative().BucketCounts()
		newNegativeBucketCounts := make([]int64, 0, negativeBucketCounts.Len())
		for j := 0; j < negativeBucketCounts.Len(); j++ {
			newNegativeBucketCounts = append(newNegativeBucketCounts, int64(negativeBucketCounts.At(j)))
		}

		metric := &dMetricExponentialHistogram{
			dMetric:                dm,
			Timestamp:              e.formatTime(dp.Timestamp().AsTime()),
			Attributes:             dp.Attributes().AsRaw(),
			StartTime:              e.formatTime(dp.StartTimestamp().AsTime()),
			Count:                  int64(dp.Count()),
			Sum:                    dp.Sum(),
			Scale:                  dp.Scale(),
			ZeroCount:              int64(dp.ZeroCount()),
			PositiveOffset:         dp.Positive().Offset(),
			PositiveBucketCounts:   newPositiveBucketCounts,
			NegativeOffset:         dp.Negative().Offset(),
			NegativeBucketCounts:   newNegativeBucketCounts,
			Exemplars:              newExemplars,
			Min:                    dp.Min(),
			Max:                    dp.Max(),
			ZeroThreshold:          dp.ZeroThreshold(),
			AggregationTemporality: pm.ExponentialHistogram().AggregationTemporality().String(),
		}
		m.data = append(m.data, metric)
	}

	return nil
}
