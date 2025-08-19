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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type m2l struct { core *core; next consumer.Logs }

func newMetricsToLogs(params connector.Settings, cfg component.Config, next consumer.Logs) (*m2l, error) { c, err := newCore(params.TelemetrySettings, cfg.(*Config)); if err != nil { return nil, err } ; return &m2l{core:c, next:next}, nil }

func (m *m2l) Start(_ context.Context, _ component.Host) error { m.core.start = time.Now(); return nil }

func (m *m2l) Shutdown(_ context.Context) error { return nil }

func (m *m2l) Capabilities() consumer.Capabilities { return consumer.Capabilities{MutatesData:false} }

func (m *m2l) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error { if !m.core.cfg.Routes.ToLogs { return nil } ; trans := m.core.eng.EvaluateMetrics(md, m.core.store); if len(trans)==0 { return nil } ; out := plog.NewLogs(); rl := out.ResourceLogs().AppendEmpty(); sl := rl.ScopeLogs().AppendEmpty(); recs := sl.LogRecords(); nowObs := pcommon.NewTimestampFromTime(time.Now()); for _, tr := range trans { if !m.core.gov.Allow() { continue } ; sev := tr.Severity ; if sev=="" { sev = mapSeverity(m.core.cfg.Severity, tr.Reason, tr.RuleID) } ; lr := recs.AppendEmpty(); lr.SetObservedTimestamp(nowObs); lr.SetTimestamp(pcommon.Timestamp(tr.At.UnixNano())); lr.Body().SetStr("alert.event"); a := lr.Attributes(); a.PutStr("rule_id", tr.RuleID); a.PutStr("fingerprint", tr.Fingerprint); a.PutStr("signal", tr.Signal); a.PutStr("from", tr.From); a.PutStr("to", tr.To); a.PutStr("severity", sev); if tr.Reason!="" { a.PutStr("reason", tr.Reason) } ; putLabels(a, tr.Labels, m.core.cfg.MaxCardinality) } ; return m.next.ConsumeLogs(ctx, out) }
