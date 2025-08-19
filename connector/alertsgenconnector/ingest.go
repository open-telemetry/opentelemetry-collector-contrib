package alertsgenconnector

import (
    "time"
    "regexp"
    "go.opentelemetry.io/collector/pdata/ptrace"
    "go.opentelemetry.io/collector/pdata/plog"
    "go.opentelemetry.io/collector/pdata/pcommon"
    "go.opentelemetry.io/collector/pdata/pmetric"
)

type traceRow struct {
    ts time.Time
    attrs map[string]string
    durationNs float64
    statusCode string
}

type logRow struct {
    ts time.Time
    attrs map[string]string
    severity string
}

type metricRow struct {
    ts time.Time
    attrs map[string]string
    name string
    value float64
}

type ingester struct {
    traces []traceRow
    logs   []logRow
    metrics []metricRow
}

func newIngester() *ingester { return &ingester{} }

func (i *ingester) drain() (tr []traceRow, lg []logRow, mt []metricRow) {
    tr, lg, mt = i.traces, i.logs, i.metrics
    i.traces, i.logs, i.metrics = nil, nil, nil
    return
}

func attrsToMap(ms pcommon.Map) map[string]string {
    out := make(map[string]string, ms.Len())
    ms.Range(func(k string, v pcommon.Value) bool {
        out[k] = v.AsString()
        return true
    })
    return out
}

func (i *ingester) consumeTraces(td ptrace.Traces) error {
    rms := td.ResourceSpans()
    for r := 0; r < rms.Len(); r++ {
        rs := rms.At(r)
        rattrs := attrsToMap(rs.Resource().Attributes())
        ss := rs.ScopeSpans()
        for s := 0; s < ss.Len(); s++ {
            spans := ss.At(s).Spans()
            for j := 0; j < spans.Len(); j++ {
                sp := spans.At(j)
                dur := float64(sp.EndTimestamp()-sp.StartTimestamp()) // nanoseconds
                row := traceRow{
                    ts: sp.EndTimestamp().AsTime(),
                    attrs: map[string]string{},
                    durationNs: dur,
                    statusCode: sp.Status().Code().String(),
                }
                for k,v := range rattrs { row.attrs[k]=v }
                sp.Attributes().Range(func(k string, v pcommon.Value) bool {
                    row.attrs[k] = v.AsString()
                    return true
                })
                i.traces = append(i.traces, row)
            }
        }
    }
    return nil
}

func (i *ingester) consumeLogs(ld plog.Logs) error {
    rl := ld.ResourceLogs()
    for r := 0; r < rl.Len(); r++ {
        rs := rl.At(r)
        rattrs := attrsToMap(rs.Resource().Attributes())
        sl := rs.ScopeLogs()
        for s := 0; s < sl.Len(); s++ {
            recs := sl.At(s).LogRecords()
            for j := 0; j < recs.Len(); j++ {
                lr := recs.At(j)
                row := logRow{
                    ts: lr.Timestamp().AsTime(),
                    attrs: map[string]string{},
                    severity: lr.SeverityText(),
                }
                for k,v := range rattrs { row.attrs[k]=v }
                lr.Attributes().Range(func(k string, v pcommon.Value) bool {
                    row.attrs[k] = v.AsString()
                    return true
                })
                i.logs = append(i.logs, row)
            }
        }
    }
    return nil
}

func (i *ingester) consumeMetrics(md pmetric.Metrics) error {
    rms := md.ResourceMetrics()
    for r := 0; r < rms.Len(); r++ {
        rs := rms.At(r)
        rattrs := attrsToMap(rs.Resource().Attributes())
        sms := rs.ScopeMetrics()
        for s := 0; s < sms.Len(); s++ {
            mets := sms.At(s).Metrics()
            for m := 0; m < mets.Len(); m++ {
                met := mets.At(m)
                switch met.Type() {
                case pmetric.MetricTypeGauge:
                    dps := met.Gauge().DataPoints()
                    for d := 0; d < dps.Len(); d++ {
                        dp := dps.At(d)
                        row := metricRow{ ts: dp.Timestamp().AsTime(), attrs: map[string]string{}, name: met.Name(), value: dp.DoubleValue() }
                        for k,v := range rattrs { row.attrs[k]=v }
                        dp.Attributes().Range(func(k string, v pcommon.Value) bool { row.attrs[k]=v.AsString(); return true })
                        i.metrics = append(i.metrics, row)
                    }
                case pmetric.MetricTypeSum:
                    dps := met.Sum().DataPoints()
                    for d := 0; d < dps.Len(); d++ {
                        dp := dps.At(d)
                        row := metricRow{ ts: dp.Timestamp().AsTime(), attrs: map[string]string{}, name: met.Name(), value: dp.DoubleValue() }
                        for k,v := range rattrs { row.attrs[k]=v }
                        dp.Attributes().Range(func(k string, v pcommon.Value) bool { row.attrs[k]=v.AsString(); return true })
                        i.metrics = append(i.metrics, row)
                    }
                }
            }
        }
    }
    return nil
}

func matchAll(attrs map[string]string, sel map[string]string) bool {
    for k, pattern := range sel {
        v := attrs[k]
        ok, _ := regexp.MatchString(pattern, v)
        if !ok {
            return false
        }
    }
    return true
}
