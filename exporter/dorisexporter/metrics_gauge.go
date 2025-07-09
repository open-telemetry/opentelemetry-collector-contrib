// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	_ "embed"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

//go:embed sql/metrics_gauge_ddl.sql
var metricsGaugeDDL string

// dMetricGauge Gauge Metric to Doris
type dMetricGauge struct {
	*dMetric   `json:",inline"`
	Timestamp  string         `json:"timestamp"`
	Attributes map[string]any `json:"attributes"`
	StartTime  string         `json:"start_time"`
	Value      float64        `json:"value"`
	Exemplars  []*dExemplar   `json:"exemplars"`
}

type metricModelGauge struct {
	metricModelCommon[dMetricGauge]
}

func (m *metricModelGauge) metricType() pmetric.MetricType {
	return pmetric.MetricTypeGauge
}

func (m *metricModelGauge) tableSuffix() string {
	return "_gauge"
}

func (m *metricModelGauge) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter) error {
	if pm.Type() != pmetric.MetricTypeGauge {
		return fmt.Errorf("metric type is not gauge: %v", pm.Type().String())
	}

	dataPoints := pm.Gauge().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		exemplars := dp.Exemplars()
		newExemplars := make([]*dExemplar, 0, exemplars.Len())
		for j := 0; j < exemplars.Len(); j++ {
			exemplar := exemplars.At(j)

			newExemplar := &dExemplar{
				FilteredAttributes: exemplar.FilteredAttributes().AsRaw(),
				Timestamp:          e.formatTime(exemplar.Timestamp().AsTime()),
				Value:              e.getExemplarValue(exemplar),
				SpanID:             exemplar.SpanID().String(),
				TraceID:            exemplar.TraceID().String(),
			}

			newExemplars = append(newExemplars, newExemplar)
		}

		metric := &dMetricGauge{
			dMetric:    dm,
			Timestamp:  e.formatTime(dp.Timestamp().AsTime()),
			Attributes: dp.Attributes().AsRaw(),
			StartTime:  e.formatTime(dp.StartTimestamp().AsTime()),
			Value:      e.getNumberDataPointValue(dp),
			Exemplars:  newExemplars,
		}
		m.data = append(m.data, metric)
	}

	return nil
}
