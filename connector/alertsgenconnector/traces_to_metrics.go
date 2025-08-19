// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"context"
	"time"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type t2m struct { core *core; next consumer.Metrics }

func newTracesToMetrics(params connector.Settings, cfg component.Config, next consumer.Metrics) (*t2m, error) { c, err := newCore(params.TelemetrySettings, cfg.(*Config)); if err != nil { return nil, err } ; return &t2m{core:c, next:next}, nil }

func (t *t2m) Start(_ context.Context, _ component.Host) error { t.core.start = time.Now(); return nil }

func (t *t2m) Shutdown(_ context.Context) error { return nil }

func (t *t2m) Capabilities() consumer.Capabilities { return consumer.Capabilities{MutatesData:false} }

func (t *t2m) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if !t.core.cfg.Routes.ToMetrics { return nil }
	trans := t.core.eng.EvaluateTraces(td, t.core.store); if len(trans)==0 { return nil }
	out := pmetric.NewMetrics(); rm := out.ResourceMetrics().AppendEmpty(); sm := rm.ScopeMetrics().AppendEmpty(); metrics := sm.Metrics()
	mtotal := metrics.AppendEmpty(); mtotal.SetEmptySum(); mtotal.SetName("alerts_total"); mtotal.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative); mtotal.Sum().SetIsMonotonic(true)
	mactive := metrics.AppendEmpty(); mactive.SetEmptyGauge(); mactive.SetName("alerts_active")
	mdur := metrics.AppendEmpty(); mdur.SetEmptySum(); mdur.SetName("alerts_duration_ns"); mdur.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative); mdur.Sum().SetIsMonotonic(true)
	now := pcommon.NewTimestampFromTime(time.Now())
	keyRS := func(ruleID, signal, severity string) string { return ruleID+"|"+signal+"|"+severity }
	keyFP := func(ruleID, signal, fp string) string { return ruleID+"|"+signal+"|"+fp }
	for _, tr := range trans { if !t.core.gov.Allow() { continue } ; sev := tr.Severity ; if sev=="" { sev = mapSeverity(t.core.cfg.Severity, tr.Reason, tr.RuleID) } ; rsKey := keyRS(tr.RuleID, tr.Signal, sev) ; fpKey := keyFP(tr.RuleID, tr.Signal, tr.Fingerprint)
		{ dp := mtotal.Sum().DataPoints().AppendEmpty(); dp.SetStartTimestamp(t.startTS()); dp.SetTimestamp(now); a := dp.Attributes(); a.PutStr("rule_id", tr.RuleID); a.PutStr("signal", tr.Signal); a.PutStr("severity", sev); dp.SetIntValue(1) }
		switch tr.To { case "firing": t.core.sinceByFP[fpKey] = tr.At; t.core.activeByKey[rsKey]++; dp := mactive.Gauge().DataPoints().AppendEmpty(); dp.SetTimestamp(now); a := dp.Attributes(); a.PutStr("rule_id", tr.RuleID); a.PutStr("signal", tr.Signal); a.PutStr("severity", sev); dp.SetIntValue(t.core.activeByKey[rsKey]) ; case "resolved": if since, ok := t.core.sinceByFP[fpKey]; ok { delete(t.core.sinceByFP, fpKey); d := tr.At.Sub(since); if d > 0 { dp := mdur.Sum().DataPoints().AppendEmpty(); dp.SetStartTimestamp(t.startTS()); dp.SetTimestamp(now); a := dp.Attributes(); a.PutStr("rule_id", tr.RuleID); a.PutStr("signal", tr.Signal); a.PutStr("severity", sev); dp.SetIntValue(d.Nanoseconds()) } } ; if cur := t.core.activeByKey[rsKey]; cur > 0 { t.core.activeByKey[rsKey] = cur - 1 } ; dp := mactive.Gauge().DataPoints().AppendEmpty(); dp.SetTimestamp(now); a := dp.Attributes(); a.PutStr("rule_id", tr.RuleID); a.PutStr("signal", tr.Signal); a.PutStr("severity", sev); dp.SetIntValue(t.core.activeByKey[rsKey]) }
	}
	return t.next.ConsumeMetrics(ctx, out) }

func (t *t2m) startTS() pcommon.Timestamp { if !t.core.start.IsZero() { return pcommon.NewTimestampFromTime(t.core.start) } ; return pcommon.NewTimestampFromTime(time.Now()) }
