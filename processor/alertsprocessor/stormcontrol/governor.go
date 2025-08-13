package stormcontrol

import (
	"time"

	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/evaluation"
)

// Config is local to avoid importing the parent processor package.
// It is mapped from parent config in processor.go.
type Config struct {
	// Soft caps to detect stormy conditions.
	MaxActiveAlerts    int // if total active instances > this, slow down
	MaxAlertsPerMinute int // if estimated APM > this, slow down
	// Optional: fraction between 0..1 that gates recovery/backoff sensitivity (default 0.8)
	CircuitBreakerThreshold float64

	// Optional tunables (with defaults set in New):
	MinInterval   time.Duration // floor for evaluation interval (default: 1s)
	MaxInterval   time.Duration // ceiling for evaluation interval (default: 1m)
	BackoffFactor float64       // multiply interval by this when storming (default: 2.0)
	RecoverFactor float64       // multiply interval by this when recovering (default: 0.8)
}

// Governor adaptively adjusts the evaluation ticker to reduce alert storms.
type Governor struct {
	cfg Config

	// Baseline & current interval tracking (observed, not configured).
	baseline     time.Duration
	current      time.Duration
	lastEvalAt   time.Time
	emaPerEval   float64 // exponential moving average of alerts per evaluation
	emaSmoothing float64 // 0..1, higher = smoother; derived from CircuitBreakerThreshold
	min, max     time.Duration
	backoff, rec float64
	apmThreshold float64 // cached float form for comparators
	activeThresh int
}

// New returns a Governor with sensible defaults if fields are unset.
func New(cfg Config) *Governor {
	g := &Governor{cfg: cfg}

	// Defaults
	if cfg.MinInterval <= 0 {
		g.min = time.Second
	} else {
		g.min = cfg.MinInterval
	}
	if cfg.MaxInterval <= 0 {
		g.max = time.Minute
	} else {
		g.max = cfg.MaxInterval
	}
	if cfg.BackoffFactor <= 0 {
		g.backoff = 2.0
	} else {
		g.backoff = cfg.BackoffFactor
	}
	if cfg.RecoverFactor <= 0 || cfg.RecoverFactor >= 1 {
		g.rec = 0.8
	} else {
		g.rec = cfg.RecoverFactor
	}
	if cfg.CircuitBreakerThreshold <= 0 || cfg.CircuitBreakerThreshold >= 1 {
		g.emaSmoothing = 0.8
	} else {
		// convert "threshold" to smoothing (higher threshold ⇒ smoother)
		g.emaSmoothing = 0.6 + 0.4*cfg.CircuitBreakerThreshold
	}

	g.activeThresh = cfg.MaxActiveAlerts
	g.apmThreshold = float64(cfg.MaxAlertsPerMinute)

	return g
}

// Adapt inspects the current results and may replace the ticker with a slower/faster one.
// Pass the address of the ticker you own (e.g., &p.tick).
func (g *Governor) Adapt(tk **time.Ticker, results []evaluation.Result, ts time.Time) {
	if tk == nil || *tk == nil {
		return
	}

	// Initialize baselines on first call.
	if g.current == 0 {
		// Guess the baseline interval from the previous tick distance if we can;
		// otherwise, use the ticker's current channel behavior (not directly readable),
		// so we fall back to a sane default (10s).
		g.current = 10 * time.Second
		g.baseline = g.current
	}
	if !g.lastEvalAt.IsZero() {
		observed := ts.Sub(g.lastEvalAt)
		// clamp observed interval into [min, max] and update the running baseline
		if observed > 0 {
			if observed < g.min {
				observed = g.min
			}
			if observed > g.max {
				observed = g.max
			}
			// keep a gentle blend toward observed as our "current" cadence
			g.current = time.Duration(0.7*float64(g.current) + 0.3*float64(observed))
			if g.baseline == 0 {
				g.baseline = g.current
			}
		}
	}
	g.lastEvalAt = ts

	// Compute pressure signals.
	active := 0
	totalInst := 0
	for i := range results {
		active += len(results[i].Instances)
		totalInst += len(results[i].Instances)
	}

	// EMA of alerts-per-evaluation.
	perEval := float64(totalInst)
	if g.emaPerEval == 0 {
		g.emaPerEval = perEval
	} else {
		a := g.emaSmoothing
		g.emaPerEval = a*g.emaPerEval + (1-a)*perEval
	}

	// Estimate alerts per minute from EMA and observed cadence.
	interval := g.current
	if interval <= 0 {
		interval = 10 * time.Second
	}
	apm := g.emaPerEval * (float64(time.Minute) / float64(interval))

	// Decide whether to back off or recover.
	needBackoff := false
	if g.activeThresh > 0 && active > g.activeThresh {
		needBackoff = true
	}
	if g.apmThreshold > 0 && apm > g.apmThreshold {
		needBackoff = true
	}

	var next time.Duration
	switch {
	case needBackoff:
		next = time.Duration(float64(interval) * g.backoff)
		if next > g.max {
			next = g.max
		}
	default:
		// Recover toward baseline; do not go below min.
		target := g.baseline
		if target == 0 {
			target = interval
		}
		// Move a step toward target using recover factor.
		nextF := float64(interval)
		baseF := float64(target)
		if nextF > baseF {
			nextF = nextF * g.rec
			if nextF < baseF {
				nextF = baseF
			}
		} else {
			// already <= baseline; allow slight nudge down but never below min
			nextF = nextF * g.rec
			if nextF < float64(g.min) {
				nextF = float64(g.min)
			}
		}
		next = time.Duration(nextF)
	}

	// Only replace the ticker if interval meaningfully changed (≥ 10%).
	diff := float64(next) / float64(interval)
	if diff > 1.10 || diff < 0.90 {
		(*tk).Stop()
		*tk = time.NewTicker(next)
		g.current = next
	}
}
