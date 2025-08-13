package stormcontrol

import (
	"testing"
	"time"

	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/evaluation"
)

// helper: build N active instances
func makeResults(ruleID string, n int) []evaluation.Result {
	insts := make([]evaluation.Instance, 0, n)
	for i := 0; i < n; i++ {
		insts = append(insts, evaluation.Instance{
			RuleID:      ruleID,
			Fingerprint: ruleID + "-" + time.Now().String(),
			Active:      true,
			Value:       1,
			Labels:      map[string]string{"rule_id": ruleID},
		})
	}
	return []evaluation.Result{{
		Rule:      evaluation.Rule{ID: ruleID, Signal: "logs"},
		Signal:    "logs",
		Instances: insts,
	}}
}

func TestGovernor_Backoff_OnStorm(t *testing.T) {
	// Configure tight bounds so we can assert quickly.
	cfg := Config{
		MaxActiveAlerts:    2,                    // backoff if >2 active
		MaxAlertsPerMinute: 0,                    // ignore apm path in this test
		MinInterval:        5 * time.Millisecond, // clamp floor
		MaxInterval:        200 * time.Millisecond,
		BackoffFactor:      2.0,
		RecoverFactor:      0.8,
	}

	g := New(cfg)
	tk := time.NewTicker(10 * time.Millisecond)
	defer tk.Stop()

	ts1 := time.Now()
	// First call: lastEvalAt == zero; governor will initialize current to 10s,
	// but will clamp the next interval by MaxInterval (200ms) when backing off.
	results := makeResults("r1", 5) // > MaxActiveAlerts => backoff
	g.Adapt(&tk, results, ts1)

	if g.current == 0 {
		t.Fatalf("expected governor to set current interval after first Adapt")
	}
	if g.current > cfg.MaxInterval {
		t.Fatalf("current interval should be clamped to MaxInterval; got %v", g.current)
	}
	if g.current != cfg.MaxInterval {
		// With the large initial current (10s) and BackoffFactor 2.0, next would be 20s,
		// clamped to MaxInterval (200ms).
		t.Fatalf("expected current interval==MaxInterval (%v) after backoff, got %v", cfg.MaxInterval, g.current)
	}
}

func TestGovernor_Recovery_TowardBaseline(t *testing.T) {
	cfg := Config{
		MaxActiveAlerts:    2,
		MaxAlertsPerMinute: 0,
		MinInterval:        5 * time.Millisecond,
		MaxInterval:        200 * time.Millisecond,
		BackoffFactor:      2.0,
		RecoverFactor:      0.5, // recover faster for the test
	}
	g := New(cfg)
	tk := time.NewTicker(10 * time.Millisecond)
	defer tk.Stop()

	// Force a backoff first.
	ts1 := time.Now()
	g.Adapt(&tk, makeResults("r1", 5), ts1)
	if g.current != cfg.MaxInterval {
		t.Fatalf("prep: expected backoff to MaxInterval, got %v", g.current)
	}

	// Now simulate a later evaluation with no activity; the governor should
	// recover by decreasing the interval (toward baseline/min).
	ts2 := ts1.Add(50 * time.Millisecond)
	prev := g.current
	g.Adapt(&tk, nil, ts2)

	if g.current >= prev {
		t.Fatalf("expected recovery to decrease current interval (prev %v, now %v)", prev, g.current)
	}
	if g.current < cfg.MinInterval {
		t.Fatalf("recovery should not go below MinInterval; got %v", g.current)
	}
}
