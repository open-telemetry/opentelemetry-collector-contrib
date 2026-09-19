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

//go:embed sql/metrics_sum_ddl.sql
var metricsSumDDL string

// dMetricSum Sum Metric to Doris
type dMetricSum struct {
	*dMetric               `json:",inline"`
	Timestamp              string         `json:"timestamp"`
	Attributes             map[string]any `json:"attributes"`
	StartTime              string         `json:"start_time"`
	Value                  float64        `json:"value"`
	Exemplars              []*dExemplar   `json:"exemplars"`
	AggregationTemporality string         `json:"aggregation_temporality"`
	IsMonotonic            bool           `json:"is_monotonic"`
}

type metricModelSum struct {
	metricModelCommon[dMetricSum]
}

func (*metricModelSum) metricType() pmetric.MetricType {
	return pmetric.MetricTypeSum
}

func (*metricModelSum) tableSuffix() string {
	return "_sum"
}

func (m *metricModelSum) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter) error {
	if pm.Type() != pmetric.MetricTypeSum {
		return fmt.Errorf("metric type is not sum: %v", pm.Type().String())
	}

	dataPoints := pm.Sum().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		if dp.Flags().NoRecordedValue() {
			e.logger.Warn("dropping sum datapoint with NoRecordedValue flag", zap.String("metric_name", dm.MetricName))
			continue
		}

		if dp.ValueType() == pmetric.NumberDataPointValueTypeDouble {
			v := dp.DoubleValue()
			if math.IsNaN(v) || math.IsInf(v, 0) {
				e.logger.Warn("dropping sum datapoint with non-finite value", zap.String("metric_name", dm.MetricName), zap.Float64("value", v))
				continue
			}
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

		metric := &dMetricSum{
			dMetric:                dm,
			Timestamp:              e.formatTime(dp.Timestamp().AsTime()),
			Attributes:             dp.Attributes().AsRaw(),
			StartTime:              e.formatTime(dp.StartTimestamp().AsTime()),
			Value:                  e.getNumberDataPointValue(dp),
			Exemplars:              newExemplars,
			AggregationTemporality: pm.Sum().AggregationTemporality().String(),
			IsMonotonic:            pm.Sum().IsMonotonic(),
		}
		m.data = append(m.data, metric)
	}

	return nil
}
