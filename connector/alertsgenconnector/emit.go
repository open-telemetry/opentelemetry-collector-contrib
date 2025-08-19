
package alertsgenconnector

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func buildLogs(events []alertEvent) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	recs := sl.LogRecords()
	now := pcommon.NewTimestampFromTime(time.Now())

	for _, e := range events {
		lr := recs.AppendEmpty()
		lr.SetTimestamp(now)
		lr.Body().SetStr(e.Rule + ":" + e.State)
		lr.SetSeverityText(e.Severity)
		attrs := lr.Attributes()
		for k, v := range e.Labels {
			attrs.PutStr(k, v)
		}
		attrs.PutStr("window", e.Window)
		attrs.PutStr("for", e.For)
		attrs.PutDouble("value", e.Value)
	}
	return ld
}

type metricView = stateMetric

func buildMetrics(mv []metricView) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// active gauge
	mAct := sm.Metrics().AppendEmpty()
	mAct.SetName("otel_alert_active")
	mAct.SetEmptyGauge()
	ga := mAct.Gauge().DataPoints()

	// last value as gauge as well
	mVal := sm.Metrics().AppendEmpty()
	mVal.SetName("otel_alert_last_value")
	mVal.SetEmptyGauge()
	gv := mVal.Gauge().DataPoints()

	now := pcommon.NewTimestampFromTime(time.Now())
	for _, s := range mv {
		dp := ga.AppendEmpty()
		dp.SetTimestamp(now)
		dp.SetIntValue(s.Active)
		for k, v := range s.Labels {
			dp.Attributes().PutStr(k, v)
		}
		dp.Attributes().PutStr("rule", s.Rule)
		dp.Attributes().PutStr("severity", s.Severity)

		dp2 := gv.AppendEmpty()
		dp2.SetTimestamp(now)
		dp2.SetDoubleValue(s.LastValue)
		for k, v := range s.Labels {
			dp2.Attributes().PutStr(k, v)
		}
		dp2.Attributes().PutStr("rule", s.Rule)
		dp2.Attributes().PutStr("severity", s.Severity)
	}
	return md
}
