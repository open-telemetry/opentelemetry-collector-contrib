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

//go:embed sql/metrics_histogram_ddl.sql
var metricsHistogramDDL string

// dMetricHistogram Histogram Metric to Doris
type dMetricHistogram struct {
	*dMetric               `json:",inline"`
	Timestamp              string         `json:"timestamp"`
	Attributes             map[string]any `json:"attributes"`
	StartTime              string         `json:"start_time"`
	Count                  int64          `json:"count"`
	Sum                    float64        `json:"sum"`
	BucketCounts           []int64        `json:"bucket_counts"`
	ExplicitBounds         []float64      `json:"explicit_bounds"`
	Exemplars              []*dExemplar   `json:"exemplars"`
	Min                    float64        `json:"min"`
	Max                    float64        `json:"max"`
	AggregationTemporality string         `json:"aggregation_temporality"`
}

type metricModelHistogram struct {
	metricModelCommon[dMetricHistogram]
}

func (*metricModelHistogram) metricType() pmetric.MetricType {
	return pmetric.MetricTypeHistogram
}

func (*metricModelHistogram) tableSuffix() string {
	return "_histogram"
}

func (m *metricModelHistogram) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter) error {
	if pm.Type() != pmetric.MetricTypeHistogram {
		return fmt.Errorf("metric type is not histogram: %v", pm.Type().String())
	}

	dataPoints := pm.Histogram().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		if dp.Flags().NoRecordedValue() {
			e.logger.Warn("dropping histogram datapoint with NoRecordedValue flag", zap.String("metric_name", dm.MetricName))
			continue
		}

		if dp.HasSum() && (math.IsNaN(dp.Sum()) || math.IsInf(dp.Sum(), 0)) {
			e.logger.Warn("dropping histogram datapoint with non-finite sum", zap.String("metric_name", dm.MetricName), zap.Float64("sum", dp.Sum()))
			continue
		}
		if dp.HasMin() && (math.IsNaN(dp.Min()) || math.IsInf(dp.Min(), 0)) {
			e.logger.Warn("dropping histogram datapoint with non-finite min", zap.String("metric_name", dm.MetricName), zap.Float64("min", dp.Min()))
			continue
		}
		if dp.HasMax() && (math.IsNaN(dp.Max()) || math.IsInf(dp.Max(), 0)) {
			e.logger.Warn("dropping histogram datapoint with non-finite max", zap.String("metric_name", dm.MetricName), zap.Float64("max", dp.Max()))
			continue
		}

		explicitBounds := dp.ExplicitBounds()
		hasInvalidBound := false
		for j := 0; j < explicitBounds.Len(); j++ {
			v := explicitBounds.At(j)
			if math.IsNaN(v) || math.IsInf(v, 0) {
				e.logger.Warn("dropping histogram datapoint with non-finite explicit bound", zap.String("metric_name", dm.MetricName), zap.Float64("bound", v))
				hasInvalidBound = true
				break
			}
		}
		if hasInvalidBound {
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

		bucketCounts := dp.BucketCounts()
		newBucketCounts := make([]int64, 0, bucketCounts.Len())
		for j := 0; j < bucketCounts.Len(); j++ {
			newBucketCounts = append(newBucketCounts, int64(bucketCounts.At(j)))
		}

		newExplicitBounds := make([]float64, 0, explicitBounds.Len())
		for j := 0; j < explicitBounds.Len(); j++ {
			newExplicitBounds = append(newExplicitBounds, explicitBounds.At(j))
		}

		metric := &dMetricHistogram{
			dMetric:                dm,
			Timestamp:              e.formatTime(dp.Timestamp().AsTime()),
			Attributes:             dp.Attributes().AsRaw(),
			StartTime:              e.formatTime(dp.StartTimestamp().AsTime()),
			Count:                  int64(dp.Count()),
			Sum:                    dp.Sum(),
			BucketCounts:           newBucketCounts,
			ExplicitBounds:         newExplicitBounds,
			Exemplars:              newExemplars,
			Min:                    dp.Min(),
			Max:                    dp.Max(),
			AggregationTemporality: pm.Histogram().AggregationTemporality().String(),
		}
		m.data = append(m.data, metric)
	}

	return nil
}
