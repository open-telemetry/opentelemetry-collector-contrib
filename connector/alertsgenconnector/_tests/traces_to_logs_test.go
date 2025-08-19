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
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

type fakeEngineTL struct{ trans []Transition }

func (f *fakeEngineTL) EvaluateTraces(_ ptrace.Traces, _ interface{}) []Transition { return f.trans }

func (f *fakeEngineTL) EvaluateMetrics(_ interface{}, _ interface{}) []Transition { return nil }

func TestTracesToLogs_Basic(t *testing.T) { t0 := time.Now().Add(-2*time.Second).Round(0); fake := &fakeEngineTL{ trans: []Transition{ { Signal:"traces", RuleID:"r42", Fingerprint:"fp-xyz", From:"inactive", To:"firing", Labels: map[string]string{"svc":"checkout"}, At:t0, Reason:"spike", Severity:"warning" } } } ; sink := new(consumertest.LogsSink); cfg := createDefaultConfig().(*Config); cfg.Routes.ToLogs = true; params := connector.Settings{ TelemetrySettings: processortest.NewNopTelemetrySettings(), ID: connector.NewID(Type) } ; c, err := newTracesToLogs(params, cfg, sink); if err != nil { t.Fatalf("newTracesToLogs: %v", err) } ; c.core.eng = fake ; if err := c.ConsumeTraces(context.Background(), ptrace.NewTraces()); err != nil { t.Fatalf("ConsumeTraces: %v", err) } ; batches := sink.AllLogs(); if len(batches)!=1 { t.Fatalf("want 1 batch, got %d", len(batches)) } ; lr := firstLogRecord(batches[0]); a := lr.Attributes(); if a.Get("rule_id").Str()!="r42" || a.Get("fingerprint").Str()!="fp-xyz" || a.Get("signal").Str()!="traces" || a.Get("to").Str()!="firing" || a.Get("severity").Str()!="warning" { t.Fatal("unexpected attributes on log record") } ; if lr.Timestamp()==0 { t.Fatal("timestamp not set (ns)") } }

func firstLogRecord(l plog.Logs) plog.LogRecord { rl := l.ResourceLogs(); sl := rl.At(0).ScopeLogs(); return sl.At(0).LogRecords().At(0) }
