// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"context"
	"testing"
	"time"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

type fakeEngine struct{ trans []Transition }

func (f *fakeEngine) EvaluateTraces(_ ptrace.Traces, _ interface{}) []Transition { return f.trans }

func (f *fakeEngine) EvaluateMetrics(_ pmetric.Metrics, _ interface{}) []Transition { return nil }

func componentIDForTest() connector.ID { return connector.NewID(Type) }

func TestTracesToMetrics_AttributesAndDuration(t *testing.T) { t0 := time.Now().Add(-3*time.Second).Round(0); t1 := t0.Add(1500*time.Millisecond); fake := &fakeEngine{trans: []Transition{ { Signal:"traces", RuleID:"r1", Fingerprint:"fp-1", From:"inactive", To:"firing", Labels: map[string]string{"service.name":"pay","env":"prod"}, At:t0, Reason:"error rate high", Severity:"error" }, { Signal:"traces", RuleID:"r1", Fingerprint:"fp-1", From:"firing", To:"resolved", Labels: map[string]string{"service.name":"pay","env":"prod"}, At:t1, Reason:"recovery", Severity:"error" }, }} ; sink := new(consumertest.MetricsSink); cfg := createDefaultConfig().(*Config); cfg.Routes.ToMetrics = true; params := connector.Settings{ TelemetrySettings: processortest.NewNopTelemetrySettings(), ID: componentIDForTest() }; c, err := newTracesToMetrics(params, cfg, sink); if err != nil { t.Fatalf("newTracesToMetrics: %v", err) } ; c.core.eng = fake ; _ = c.Start(context.Background(), nil) ; if err := c.ConsumeTraces(context.Background(), ptrace.NewTraces()); err != nil { t.Fatalf("ConsumeTraces: %v", err) } ; mds := sink.AllMetrics(); if len(mds)!=1 { t.Fatalf("want 1 batch, got %d", len(mds)) } ; ms := mds[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics(); var totalDP, activeDP, durDP int; var sawTotalAttrs, sawActiveAttrs, sawDurAttrs bool; var durationVal int64; for i:=0;i<ms.Len();i++ { m := ms.At(i); switch m.Name() { case "alerts_total": dps := m.Sum().DataPoints(); totalDP = dps.Len(); for j:=0;j<dps.Len();j++ { a := dps.At(j).Attributes(); if a.Get("rule_id").Str()=="r1" && a.Get("signal").Str()=="traces" && a.Get("severity").Str()=="error" { sawTotalAttrs = true } } case "alerts_active": dps := m.Gauge().DataPoints(); activeDP = dps.Len(); for j:=0;j<dps.Len();j++ { a := dps.At(j).Attributes(); if a.Get("rule_id").Str()=="r1" && a.Get("signal").Str()=="traces" && a.Get("severity").Str()=="error" { sawActiveAttrs = true } } case "alerts_duration_ns": dps := m.Sum().DataPoints(); durDP = dps.Len(); for j:=0;j<dps.Len();j++ { a := dps.At(j).Attributes(); if a.Get("rule_id").Str()=="r1" && a.Get("signal").Str()=="traces" && a.Get("severity").Str()=="error" { sawDurAttrs = true } ; if v := dps.At(j).IntValue(); v > 0 { durationVal = v } } } } ; if totalDP != 2 { t.Fatalf("alerts_total points=%d, want 2", totalDP) } ; if activeDP != 2 { t.Fatalf("alerts_active points=%d, want 2", activeDP) } ; if durDP != 1 { t.Fatalf("alerts_duration_ns points=%d, want 1", durDP) } ; if !sawTotalAttrs || !sawActiveAttrs || !sawDurAttrs { t.Fatal("missing expected attributes") } ; if durationVal <= 0 { t.Fatalf("duration must be >0, got %d", durationVal) } }
