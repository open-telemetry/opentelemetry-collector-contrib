// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stormcontrol

import (
	"testing"
	"time"
)

// helper for Duration wrapper
func D(d time.Duration) Duration { return Duration(d) }

func TestGovernor_Backoff_OnStormActive(t *testing.T) {
	cfg, err := (Config{
		MaxActiveAlerts:    2, // back off if >2 active
		MaxAlertsPerMinute: 0, // ignore APM for this test
		MinInterval:        D(5 * time.Millisecond),
		MaxInterval:        D(200 * time.Millisecond),
		BackoffFactor:      2.0,
		RecoverFactor:      0.5,
	}).Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	g := New(cfg)
	if got := g.Current(); got != 5*time.Millisecond {
		t.Fatalf("initial current = %v, want 5ms", got)
	}

	want := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond,
		80 * time.Millisecond,
		160 * time.Millisecond,
		200 * time.Millisecond, // clamp at max
		200 * time.Millisecond, // stays clamped
	}
	for i, w := range want {
		got := g.Update(3 /*active*/, 0 /*apm*/)
		if got != w {
			t.Fatalf("step %d: got %v, want %v", i, got, w)
		}
	}
}

func TestGovernor_Recovery_FromStorm(t *testing.T) {
	cfg, err := (Config{
		MaxActiveAlerts:    1,
		MaxAlertsPerMinute: 0,
		MinInterval:        D(5 * time.Millisecond),
		MaxInterval:        D(200 * time.Millisecond),
		BackoffFactor:      2.0,
		RecoverFactor:      0.5,
	}).Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	g := New(cfg)

	// push to ceiling
	for g.Current() < 200*time.Millisecond {
		g.Update(2, 0)
	}
	if g.Current() != 200*time.Millisecond {
		t.Fatalf("expected max, got %v", g.Current())
	}

	// recovery uses ns math: 25ms -> 12.5ms -> 6.25ms -> clamp 5ms
	want := []time.Duration{
		100 * time.Millisecond,
		50 * time.Millisecond,
		25 * time.Millisecond,
		12*time.Millisecond + 500*time.Microsecond, // 12.5ms
		6*time.Millisecond + 250*time.Microsecond,  // 6.25ms
		5 * time.Millisecond,                       // clamp to min
		5 * time.Millisecond,                       // stay at min
	}
	for i, w := range want {
		got := g.Update(0, 0)
		if got != w {
			t.Fatalf("recover step %d: got %v, want %v", i, got, w)
		}
	}
}

func TestGovernor_Storm_ByAPMThreshold(t *testing.T) {
	cfg, err := (Config{
		MaxActiveAlerts:    0, // disable active path
		MaxAlertsPerMinute: 50,
		MinInterval:        D(5 * time.Millisecond),
		MaxInterval:        D(200 * time.Millisecond),
		BackoffFactor:      2.0,
		RecoverFactor:      0.5,
	}).Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	g := New(cfg)
	// Make EMA respond immediately so we assert a single-tick backoff
	g.alpha = 1.0

	if g.Current() != 5*time.Millisecond {
		t.Fatalf("initial current = %v, want 5ms", g.Current())
	}
	if got := g.Update(0, 0); got != 5*time.Millisecond { // calm tick
		t.Fatalf("calm: got %v, want 5ms", got)
	}

	// APM=100 > 50 => stormy -> backoff to 10ms
	if got := g.Update(0, 100); got != 10*time.Millisecond {
		t.Fatalf("APM storm backoff: got %v, want 10ms", got)
	}
}

func TestGovernor_ClampRespectsBounds(t *testing.T) {
	cfg, err := (Config{
		MaxActiveAlerts:    1,
		MaxAlertsPerMinute: 0,
		MinInterval:        D(5 * time.Millisecond),
		MaxInterval:        D(200 * time.Millisecond),
		BackoffFactor:      10.0,
		RecoverFactor:      0.1,
	}).Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	g := New(cfg)

	if got := g.Update(2, 0); got != 50*time.Millisecond {
		t.Fatalf("expected 50ms, got %v", got)
	}
	if got := g.Update(2, 0); got != 200*time.Millisecond { // clamp
		t.Fatalf("expected 200ms, got %v", got)
	}
	if got := g.Update(0, 0); got != 20*time.Millisecond { // 200ms * 0.1
		t.Fatalf("expected 20ms, got %v", got)
	}
}

func TestConfig_Validate_Errors(t *testing.T) {
	if _, err := (Config{
		MinInterval:   D(10 * time.Millisecond),
		MaxInterval:   D(5 * time.Millisecond),
		BackoffFactor: 2.0, RecoverFactor: 0.5,
	}).Validate(); err == nil {
		t.Fatal("expected error for max_interval < min_interval")
	}
	if _, err := (Config{
		MinInterval:   D(5 * time.Millisecond),
		MaxInterval:   D(10 * time.Millisecond),
		BackoffFactor: 0.9, RecoverFactor: 0.5,
	}).Validate(); err == nil {
		t.Fatal("expected error for backoff_factor < 1.0")
	}
	if _, err := (Config{
		MinInterval:   D(5 * time.Millisecond),
		MaxInterval:   D(10 * time.Millisecond),
		BackoffFactor: 2.0, RecoverFactor: 0.0,
	}).Validate(); err == nil {
		t.Fatal("expected error for recover_factor <= 0")
	}
	if _, err := (Config{
		MinInterval:   D(5 * time.Millisecond),
		MaxInterval:   D(10 * time.Millisecond),
		BackoffFactor: 2.0, RecoverFactor: 1.1,
	}).Validate(); err == nil {
		t.Fatal("expected error for recover_factor > 1.0")
	}
}
