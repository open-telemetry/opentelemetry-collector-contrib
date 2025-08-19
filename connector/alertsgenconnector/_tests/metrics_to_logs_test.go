// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"context"
	"testing"
	"time"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
)

type fakeEngineML struct{ trans []Transition }

func (f *fakeEngineML) EvaluateMetrics(_ pmetric.Metrics, _ interface{}) []Transition { return f.trans }

func (f *fakeEngineML) EvaluateTraces(_ interface{}, _ interface{}) []Transition { return nil }

func TestMetricsToLogs_Basic(t *testing.T) { t0 := time.Now().Add(-1*time.Second).Round(0); fake := &fakeEngineML{ trans: []Transition{ { Signal:"metrics", RuleID:"r99", Fingerprint:"fp-777", From:"inactive", To:"firing", Labels: map[string]string{"workload":"ingester"}, At:t0, Reason:"threshold exceeded", Severity:"error" } } } ; sink := new(consumertest.LogsSink); cfg := createDefaultConfig().(*Config); cfg.Routes.ToLogs = true; params := connector.Settings{ TelemetrySettings: processortest.NewNopTelemetrySettings(), ID: connector.NewID(Type) } ; c, err := newMetricsToLogs(params, cfg, sink); if err != nil { t.Fatalf("newMetricsToLogs: %v", err) } ; c.core.eng = fake ; if err := c.ConsumeMetrics(context.Background(), pmetric.NewMetrics()); err != nil { t.Fatalf("ConsumeMetrics: %v", err) } ; batches := sink.AllLogs(); if len(batches)!=1 { t.Fatalf("want 1 batch, got %d", len(batches)) } ; lr := firstLogRecord(batches[0]); a := lr.Attributes(); if a.Get("rule_id").Str()!="r99" || a.Get("fingerprint").Str()!="fp-777" || a.Get("signal").Str()!="metrics" || a.Get("to").Str()!="firing" || a.Get("severity").Str()!="error" { t.Fatal("unexpected attributes") } ; if lr.Timestamp()==0 { t.Fatal("expected ns timestamp") } }

func firstLogRecord(l plog.Logs) plog.LogRecord { rl := l.ResourceLogs(); sl := rl.At(0).ScopeLogs(); return sl.At(0).LogRecords().At(0) }
