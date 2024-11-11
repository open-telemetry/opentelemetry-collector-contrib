// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	_ "embed"
	"encoding/json"
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
	data []*dMetricGauge
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
		newExeplars := make([]*dExemplar, 0, exemplars.Len())
		for j := 0; j < exemplars.Len(); j++ {
			exemplar := exemplars.At(j)

			newExeplar := &dExemplar{
				FilteredAttributes: exemplar.FilteredAttributes().AsRaw(),
				Timestamp:          e.formatTime(exemplar.Timestamp().AsTime()),
				Value:              e.getExemplarValue(exemplar),
				SpanID:             exemplar.SpanID().String(),
				TraceID:            exemplar.TraceID().String(),
			}

			newExeplars = append(newExeplars, newExeplar)
		}

		metric := &dMetricGauge{
			dMetric:    dm,
			Timestamp:  e.formatTime(dp.Timestamp().AsTime()),
			Attributes: dp.Attributes().AsRaw(),
			StartTime:  e.formatTime(dp.StartTimestamp().AsTime()),
			Value:      e.getNumberDataPointValue(dp),
			Exemplars:  newExeplars,
		}
		m.data = append(m.data, metric)
	}

	return nil
}

func (m *metricModelGauge) raw() any {
	return m.data
}

func (m *metricModelGauge) size() int {
	return len(m.data)
}

func (m *metricModelGauge) bytes() ([]byte, error) {
	return json.Marshal(m.data)
}
