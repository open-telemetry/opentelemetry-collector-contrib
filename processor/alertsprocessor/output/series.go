package output

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/evaluation"
	"github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/statestore"
)

type SeriesBuilder struct {
	commonRes map[string]string
}

func NewSeriesBuilder(external map[string]string) *SeriesBuilder {
	return &SeriesBuilder{commonRes: external}
}

func (b *SeriesBuilder) Build(results []evaluation.Result, trans []statestore.Transition, ts time.Time) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	for k, v := range b.commonRes { rm.Resource().Attributes().PutStr(k, v) }
	sm := rm.ScopeMetrics().AppendEmpty()
	metrics := sm.Metrics()

	// 1) state gauge
	mState := metrics.AppendEmpty()
	mState.SetName("otel_alert_state")
	mState.SetEmptyGauge()
	for _, r := range results {
		for _, inst := range r.Instances {
			if !inst.Active { continue }
			dp := mState.Gauge().DataPoints().AppendEmpty()
			dp.SetIntValue(1)
			dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			for k, v := range inst.Labels { dp.Attributes().PutStr(k, v) }
			dp.Attributes().PutStr("rule_id", r.Rule.ID)
			dp.Attributes().PutStr("signal", r.Signal)
		}
	}

	// 2) transitions counter
	mTrans := metrics.AppendEmpty()
	mTrans.SetName("otel_alert_transitions_total")
	mTrans.SetEmptySum()
	mTrans.Sum().SetIsMonotonic(true)
	mTrans.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	for _, t := range trans {
		dp := mTrans.Sum().DataPoints().AppendEmpty()
		dp.SetIntValue(1)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		dp.Attributes().PutStr("rule_id", t.RuleID)
		dp.Attributes().PutStr("from", t.From)
		dp.Attributes().PutStr("to", t.To)
		dp.Attributes().PutStr("signal", t.Signal)
		for k, v := range t.Labels { dp.Attributes().PutStr(k, v) }
	}

	return md
}
