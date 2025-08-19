
package alertsgenconnector

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type traceRow struct {
	ts         time.Time
	attrs      map[string]string
	durationNs float64
	statusCode string
}

type logRow struct {
	ts    time.Time
	attrs map[string]string
	body  string
	severity string
}

type metricRow struct {
	ts    time.Time
	attrs map[string]string
	name  string
	value float64
}

type ingester struct {
	traces  []traceRow
	logs    []logRow
	metrics []metricRow
}

func newIngester() *ingester { return &ingester{} }

func (i *ingester) drain() (tr []traceRow, lg []logRow, mt []metricRow) {
	tr, lg, mt = i.traces, i.logs, i.metrics
	i.traces, i.logs, i.metrics = nil, nil, nil
	return
}

func (i *ingester) consumeTraces(td ptrace.Traces) error {
	rss := td.ResourceSpans()
	for r := 0; r < rss.Len(); r++ {
		rs := rss.At(r)
		base := attrsToMap(rs.Resource().Attributes())
		ss := rs.ScopeSpans()
		for s := 0; s < ss.Len(); s++ {
			spans := ss.At(s).Spans()
			for j := 0; j < spans.Len(); j++ {
				sp := spans.At(j)
				m := map[string]string{}
				for k, v := range base { m[k]=v }
				mergeAttrs(m, sp.Attributes())
				ts := sp.StartTimestamp().AsTime()
				dur := float64(sp.EndTimestamp()-sp.StartTimestamp())
				i.traces = append(i.traces, traceRow{
					ts: ts, attrs: m, durationNs: dur, statusCode: sp.Status().Code().String(),
				})
			}
		}
	}
	return nil
}

func (i *ingester) consumeLogs(ld plog.Logs) error {
	rls := ld.ResourceLogs()
	for r := 0; r < rls.Len(); r++ {
		rl := rls.At(r)
		base := attrsToMap(rl.Resource().Attributes())
		sls := rl.ScopeLogs()
		for s := 0; s < sls.Len(); s++ {
			recs := sls.At(s).LogRecords()
			for j := 0; j < recs.Len(); j++ {
				lr := recs.At(j)
				m := map[string]string{}
				for k, v := range base { m[k]=v }
				mergeAttrs(m, lr.Attributes())
				ts := lr.Timestamp().AsTime()
				i.logs = append(i.logs, logRow{
					ts: ts, attrs: m, severity: lr.SeverityText(), body: lr.Body().AsString(),
				})
			}
		}
	}
	return nil
}

func (i *ingester) consumeMetrics(md pmetric.Metrics) error {
	rms := md.ResourceMetrics()
	for r := 0; r < rms.Len(); r++ {
		rm := rms.At(r)
		base := attrsToMap(rm.Resource().Attributes())
		sms := rm.ScopeMetrics()
		for s := 0; s < sms.Len(); s++ {
			ms := sms.At(s).Metrics()
			for j := 0; j < ms.Len(); j++ {
				m := ms.At(j)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					ds := m.Gauge().DataPoints()
					for k := 0; k < ds.Len(); k++ {
						dp := ds.At(k)
						lbls := map[string]string{}
						for k2, v := range base { lbls[k2]=v }
						for l := 0; l < dp.Attributes().Len(); l++ {
							k3 := dp.Attributes().At(l).Key()
							lbls[k3] = dp.Attributes().At(l).Value().AsString()
						}
						ts := dp.Timestamp().AsTime()
						i.metrics = append(i.metrics, metricRow{
							ts: ts, attrs: lbls, name: m.Name(), value: dp.DoubleValue(),
						})
					}
				case pmetric.MetricTypeSum:
					ds := m.Sum().DataPoints()
					for k := 0; k < ds.Len(); k++ {
						dp := ds.At(k)
						lbls := map[string]string{}
						for k2, v := range base { lbls[k2]=v }
						for l := 0; l < dp.Attributes().Len(); l++ {
							k3 := dp.Attributes().At(l).Key()
							lbls[k3] = dp.Attributes().At(l).Value().AsString()
						}
						ts := dp.Timestamp().AsTime()
						v := dp.DoubleValue()
						i.metrics = append(i.metrics, metricRow{
							ts: ts, attrs: lbls, name: m.Name(), value: v,
						})
					}
				}
			}
		}
	}
	return nil
}

func attrsToMap(ma pcommon.Map) map[string]string {
	out := map[string]string{}
	ma.Range(func(k string, v pcommon.Value) bool {
		out[k] = v.AsString()
		return true
	})
	return out
}

func mergeAttrs(dst map[string]string, m pcommon.Map) {
	m.Range(func(k string, v pcommon.Value) bool {
		dst[k] = v.AsString()
		return true
	})
}
