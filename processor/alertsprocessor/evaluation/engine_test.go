package evaluation

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func makeResWithAttr(k, v string) pcommon.Resource {
	res := pcommon.NewResource()
	res.Attributes().PutStr(k, v)
	return res
}

func TestRunLogs_CountThreshold_WithGroupBy(t *testing.T) {
	log := zap.NewNop()

	rules := []Rule{{
		ID:     "logs1",
		Name:   "HighErrors",
		Signal: "logs",
		Logs: &LogsRule{
			SeverityAtLeast: "ERROR",
			BodyContains:    "timeout",
			GroupBy:         []string{"service.name"},
			CountThreshold:  2,
		},
	}}
	e := NewEngine(Sources{Inline: rules}, log)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "svc-a")
	sl := rl.ScopeLogs().AppendEmpty()
	lrs := sl.LogRecords()

	// two matching records
	for i := 0; i < 2; i++ {
		lr := lrs.AppendEmpty()
		lr.SetSeverityNumber(plog.SeverityNumberError)
		lr.Body().SetStr("timeout while calling backend")
	}

	out := e.RunLogs([]plog.Logs{ld}, time.Now())
	if len(out) != 1 || len(out[0].Instances) != 1 {
		t.Fatalf("expected 1 result / 1 instance, got %d/%d", len(out), func() int {
			if len(out) == 0 {
				return 0
			}
			return len(out[0].Instances)
		}())
	}
	if got := out[0].Instances[0].Value; got != 2 {
		t.Fatalf("expected count=2, got %v", got)
	}
}

func TestRunMetrics_Gauge_Count(t *testing.T) {
	log := zap.NewNop()
	rules := []Rule{{
		ID:     "m1",
		Name:   "GaugeCount",
		Signal: "metrics",
		Metrics: &MetricsRule{
			MetricName:     "app.errors",
			Aggregation:    "count",
			CountThreshold: 2,
		},
	}}
	e := NewEngine(Sources{Inline: rules}, log)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "svc1")
	sm := rm.ScopeMetrics().AppendEmpty()

	m := sm.Metrics().AppendEmpty()
	m.SetName("app.errors")
	m.SetEmptyGauge()
	dp := m.Gauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1)
	dp2 := m.Gauge().DataPoints().AppendEmpty()
	dp2.SetDoubleValue(1)

	out := e.RunMetrics([]pmetric.Metrics{md}, time.Now())
	if len(out) != 1 || len(out[0].Instances) != 1 {
		t.Fatalf("expected 1 result / 1 instance, got %d/%d", len(out), func() int {
			if len(out) == 0 {
				return 0
			}
			return len(out[0].Instances)
		}())
	}
	if out[0].Instances[0].Value != 2 {
		t.Fatalf("expected count value 2, got %v", out[0].Instances[0].Value)
	}
}
