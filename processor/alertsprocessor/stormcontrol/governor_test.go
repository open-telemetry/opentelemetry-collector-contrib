package stormcontrol

import (
	"testing"
	"time"
)

// convenience helper to build Duration values in tests
func D(d time.Duration) Duration { return Duration(d) }

func TestGovernor_Backoff_OnStormActive(t *testing.T) {
	// Tight bounds for fast assertions.
	cfg, err := (Config{
		MaxActiveAlerts:    2, // back off if >2 active
		MaxAlertsPerMinute: 0, // ignore APM path here
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

	// Repeated stormy updates should back off up to MaxInterval and then clamp.
	wantSeq := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond,
		80 * time.Millisecond,
		160 * time.Millisecond,
		200 * time.Millisecond, // clamp at max
		200 * time.Millisecond, // stays clamped
	}
	for i, want := range wantSeq {
		got := g.Update(3 /*active*/, 0 /*apm*/)
		if got != want {
			t.Fatalf("step %d: got %v, want %v", i, got, want)
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

	// First, push to the ceiling quickly.
	for g.Current() < 200*time.Millisecond {
		g.Update(2 /*storm*/, 0)
	}
	if g.Current() != 200*time.Millisecond {
		t.Fatalf("expected to reach max, got %v", g.Current())
	}

	// Now calm conditions should recover toward MinInterval, clamped at floor.
	// Note: halving time.Duration truncates toward zero.
	wantSeq := []time.Duration{
		100 * time.Millisecond,
		50 * time.Millisecond,
		25 * time.Millisecond,
		12 * time.Millisecond, // 25ms * 0.5 = 12.5ms -> 12ms after truncation
		6 * time.Millisecond,  // 12ms * 0.5 = 6ms
		5 * time.Millisecond,  // floor at min
		5 * time.Millisecond,  // stays at floor
	}
	for i, want := range wantSeq {
		got := g.Update(0 /*active*/, 0 /*apm*/)
		if got != want {
			t.Fatalf("recover step %d: got %v, want %v", i, got, want)
		}
	}
}

func TestGovernor_Storm_ByAPMThreshold(t *testing.T) {
	cfg, err := (Config{
		MaxActiveAlerts:    0, // disable active-based storming for this test
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
	if g.Current() != 5*time.Millisecond {
		t.Fatalf("initial current = %v, want 5ms", g.Current())
	}

	// Calm tick shouldn't change (already at floor).
	if got := g.Update(0 /*active*/, 0 /*apm*/); got != 5*time.Millisecond {
		t.Fatalf("calm update: got %v, want 5ms", got)
	}

	// Large APM should trigger stormy backoff via EMA on first tick.
	if got := g.Update(0 /*active*/, 100 /*apm*/); got != 10*time.Millisecond {
		t.Fatalf("APM storm backoff: got %v, want 10ms", got)
	}
}

func TestGovernor_ClampRespectsBounds(t *testing.T) {
	cfg, err := (Config{
		MaxActiveAlerts:    1,
		MaxAlertsPerMinute: 0,
		MinInterval:        D(5 * time.Millisecond),
		MaxInterval:        D(200 * time.Millisecond),
		BackoffFactor:      10.0, // aggressive
		RecoverFactor:      0.1,  // aggressive
	}).Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	g := New(cfg)

	// One stormy tick: 5ms -> 50ms.
	if got := g.Update(2, 0); got != 50*time.Millisecond {
		t.Fatalf("expected 50ms, got %v", got)
	}
	// Next stormy tick would be 500ms, but must clamp to 200ms.
	if got := g.Update(2, 0); got != 200*time.Millisecond {
		t.Fatalf("expected clamp to 200ms, got %v", got)
	}

	// Recovery from 200ms with factor 0.1 should try to go to 20ms, but floor is 5ms only after enough steps.
	// First calm tick: 200ms -> 20ms (truncation still exact here).
	if got := g.Update(0, 0); got != 20*time.Millisecond {
		t.Fatalf("expected 20ms, got %v", got)
	}
}

func TestConfig_Validate_Errors(t *testing.T) {
	// max < min
	if _, err := (Config{
		MinInterval:   D(10 * time.Millisecond),
		MaxInterval:   D(5 * time.Millisecond),
		BackoffFactor: 2.0, RecoverFactor: 0.5,
	}).Validate(); err == nil {
		t.Fatal("expected error for max_interval < min_interval")
	}

	// backoff < 1.0
	if _, err := (Config{
		MinInterval:   D(5 * time.Millisecond),
		MaxInterval:   D(10 * time.Millisecond),
		BackoffFactor: 0.9, RecoverFactor: 0.5,
	}).Validate(); err == nil {
		t.Fatal("expected error for backoff_factor < 1.0")
	}

	// recover <= 0 or > 1
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
