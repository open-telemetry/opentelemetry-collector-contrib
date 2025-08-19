package alertsgenconnector

import (
    "go.opentelemetry.io/collector/pdata/plog"
    "go.opentelemetry.io/collector/pdata/pcommon"
    "go.opentelemetry.io/collector/pdata/pmetric"
    "time"
)

func buildLogs(events []alertEvent) plog.Logs {
    ld := plog.NewLogs()
    rl := ld.ResourceLogs().AppendEmpty()
    sl := rl.ScopeLogs().AppendEmpty()
    recs := sl.LogRecords()
    now := pcommon.NewTimestampFromTime(time.Now())
    for _, e := range events {
        lr := recs.AppendEmpty()
        if e.State == "firing" {
            lr.SetSeverityText("ERROR")
        } else {
            lr.SetSeverityText("INFO")
        }
        body := lr.Body()
        body.SetStr(e.Rule + " -> " + e.State)
        lr.Attributes().PutStr("alert.rule", e.Rule)
        lr.Attributes().PutStr("alert.state", e.State)
        lr.Attributes().PutStr("alert.severity", e.Severity)
        lr.Attributes().PutStr("window", e.Window)
        lr.Attributes().PutStr("for", e.For)
        lr.Attributes().PutDouble("value", e.Value)
        for k,v := range e.Labels {
            lr.Attributes().PutStr(k, v)
        }
        lr.SetTimestamp(now)
    }
    return ld
}

func buildMetrics(states []stateMetric) pmetric.Metrics {
    md := pmetric.NewMetrics()
    rm := md.ResourceMetrics().AppendEmpty()
    sm := rm.ScopeMetrics().AppendEmpty()
    now := pcommon.NewTimestampFromTime(time.Now())
    for _, s := range states {
        m1 := sm.Metrics().AppendEmpty()
        m1.SetName("otel_alerts_active")
        g1 := m1.SetEmptyGauge()
        dp1 := g1.DataPoints().AppendEmpty()
        dp1.SetDoubleValue(float64(s.Active))
        for k,v := range s.Labels { dp1.Attributes().PutStr(k, v) }
        dp1.Attributes().PutStr("rule", s.Rule)
        dp1.Attributes().PutStr("severity", s.Severity)
        dp1.SetTimestamp(now)

        m2 := sm.Metrics().AppendEmpty()
        m2.SetName("otel_alerts_last_value")
        g2 := m2.SetEmptyGauge()
        dp2 := g2.DataPoints().AppendEmpty()
        dp2.SetDoubleValue(s.LastValue)
        for k,v := range s.Labels { dp2.Attributes().PutStr(k, v) }
        dp2.Attributes().PutStr("rule", s.Rule)
        dp2.Attributes().PutStr("severity", s.Severity)
        dp2.SetTimestamp(now)
    }
    return md
}
