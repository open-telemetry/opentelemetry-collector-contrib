// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	_ "embed"
	"fmt"

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

		exemplars := dp.Exemplars()
		newExemplars := make([]*dExemplar, 0, exemplars.Len())
		for j := 0; j < exemplars.Len(); j++ {
			exemplar := exemplars.At(j)

			value := e.getExemplarValue(exemplar)
			if !isFiniteNumber(value) {
				e.logger.Warn("dropping exemplar with non-finite value",
					zap.String("metric", pm.Name()),
					zap.Float64("value", value))
				continue
			}

			newExemplar := &dExemplar{
				FilteredAttributes: exemplar.FilteredAttributes().AsRaw(),
				Timestamp:          e.formatTime(exemplar.Timestamp().AsTime()),
				Value:              value,
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
		if !isFiniteNumber(metric.Value) {
			e.logger.Warn("dropping sum data point with non-finite value",
				zap.String("metric", pm.Name()),
				zap.Float64("value", metric.Value))
			continue
		}
		m.data = append(m.data, metric)
	}

	return nil
}
