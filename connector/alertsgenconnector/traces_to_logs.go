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
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type t2l struct { core *core; next consumer.Logs }

func newTracesToLogs(params connector.Settings, cfg component.Config, next consumer.Logs) (*t2l, error) { c, err := newCore(params.TelemetrySettings, cfg.(*Config)); if err != nil { return nil, err } ; return &t2l{core:c, next:next}, nil }

func (t *t2l) Start(_ context.Context, _ component.Host) error { t.core.start = time.Now(); return nil }

func (t *t2l) Shutdown(_ context.Context) error { return nil }

func (t *t2l) Capabilities() consumer.Capabilities { return consumer.Capabilities{MutatesData:false} }

func (t *t2l) ConsumeTraces(ctx context.Context, td ptrace.Traces) error { if !t.core.cfg.Routes.ToLogs { return nil } ; trans := t.core.eng.EvaluateTraces(td, t.core.store); if len(trans)==0 { return nil } ; out := plog.NewLogs(); rl := out.ResourceLogs().AppendEmpty(); sl := rl.ScopeLogs().AppendEmpty(); recs := sl.LogRecords(); nowObs := pcommon.NewTimestampFromTime(time.Now()); for _, tr := range trans { if !t.core.gov.Allow() { continue } ; sev := tr.Severity ; if sev=="" { sev = mapSeverity(t.core.cfg.Severity, tr.Reason, tr.RuleID) } ; lr := recs.AppendEmpty(); lr.SetObservedTimestamp(nowObs); lr.SetTimestamp(pcommon.Timestamp(tr.At.UnixNano())); lr.Body().SetStr("alert.event"); a := lr.Attributes(); a.PutStr("rule_id", tr.RuleID); a.PutStr("fingerprint", tr.Fingerprint); a.PutStr("signal", tr.Signal); a.PutStr("from", tr.From); a.PutStr("to", tr.To); a.PutStr("severity", sev); if tr.Reason!="" { a.PutStr("reason", tr.Reason) } ; putLabels(a, tr.Labels, t.core.cfg.MaxCardinality) } ; return t.next.ConsumeLogs(ctx, out) }
