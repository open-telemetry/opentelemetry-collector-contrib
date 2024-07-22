package metrics

import (
	"strings"
	"time"

	p "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/proto"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	COUNTER   string = "counter"
	GAUGE     string = "gauge"
	HISTOGRAM string = "histogram"
	SUMMARY   string = "summary"
)

func ConvertMetrics(promMetrics *p.WriteRequest, metricMap map[string]string) *pmetric.Metrics {
	if promMetrics == nil {
		return nil
	}
	otelMetrics := pmetric.NewMetrics()

	for _, ts := range promMetrics.Timeseries {
		var metricType, metricName string
		metricName = getMetricNameFromLabels(ts.Labels)

		for k, v := range metricMap {
			if strings.HasSuffix(metricName, k) { // Should we use the HasSuffix() or Contains()?
				metricType = v
				break
			}
		}

		if metricType == "" {
			continue
		}

		switch metricType {
		case COUNTER:
			convertCounter(ts, otelMetrics, metricName)
		case GAUGE:
			convertGauge(ts, otelMetrics, metricName)
		case HISTOGRAM:
			convertHistogram()
		case SUMMARY:
			convertSummary()
		default:
			continue // How do we want to handle metric types that are not currently supported?
		}
	}
	return &otelMetrics
}

func getMetricNameFromLabels(labels []p.Label) string {
	for _, label := range labels {
		if label.Name == "__name__" {
			return label.Value
		}
	}
	return ""
}

func convertCounter(timeseries p.TimeSeries, otelMetrics pmetric.Metrics, metricName string) {
	rm := otelMetrics.ResourceMetrics().AppendEmpty()

	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName(metricName)

	sum := m.SetEmptySum()
	sum.SetIsMonotonic(true)

	for _, e := range timeseries.Samples {
		t := time.Unix(e.Timestamp/1000, (e.Timestamp%1000)*1e6).UTC()

		dp := sum.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(t))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(t))
		dp.SetDoubleValue(e.Value)

		attributes := dp.Attributes()
		for _, label := range timeseries.Labels {
			attributes.PutStr(label.Name, label.Value)
		}
	}
}

func convertGauge(timeseries p.TimeSeries, otelMetrics pmetric.Metrics, metricName string) {
	rm := otelMetrics.ResourceMetrics().AppendEmpty()

	resAttribs := rm.Resource().Attributes()
	for _, label := range timeseries.Labels {
		resAttribs.PutStr(label.Name, label.Value)
	}

	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName(metricName)

	gauge := m.SetEmptyGauge()

	for _, e := range timeseries.Samples {
		t := time.Unix(e.Timestamp/1000, (e.Timestamp%1000)*1e6).UTC()

		dp := gauge.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(t))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(t))
		dp.SetDoubleValue(e.Value)

		attributes := dp.Attributes()
		for _, label := range timeseries.Labels {
			attributes.PutStr(label.Name, label.Value)
		}
	}

}

func convertHistogram() {}

func convertSummary() {}
