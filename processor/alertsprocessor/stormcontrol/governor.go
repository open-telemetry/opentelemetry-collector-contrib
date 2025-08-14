package stormcontrol

import (
	"time"
)

// Governor adapts the processor's evaluation interval within [MinInterval, MaxInterval]
// using backoff/recovery based on observed alert volume.
type Governor struct {
	cfg Config

	cur    time.Duration // current interval
	emaAPM float64       // exponential moving average of alerts-per-minute
	alpha  float64       // smoothing factor for EMA [0,1]
}

// New creates a Governor. Expect cfg to be validated already via cfg.Validate().
func New(cfg Config) *Governor {
	g := &Governor{
		cfg:   cfg,
		cur:   cfg.MinInterval.Duration(),
		alpha: 0.2, // reasonable default smoothing for APM EMA
	}
	return g
}

// Update adapts the interval based on current totals.
//   - active: number of currently active alert instances
//   - apm:    alerts per minute observed in the last evaluation
//
// Returns the next interval to use.
func (g *Governor) Update(active int, apm float64) time.Duration {
	// Smooth APM to avoid twitchiness.
	g.emaAPM = g.alpha*apm + (1.0-g.alpha)*g.emaAPM

	stormy := (g.cfg.MaxActiveAlerts > 0 && active > g.cfg.MaxActiveAlerts) ||
		(g.cfg.MaxAlertsPerMinute > 0 && int(g.emaAPM) > g.cfg.MaxAlertsPerMinute)

	switch {
	case stormy:
		g.backoff()
	default:
		g.recover()
	}
	return g.cur
}

func (g *Governor) backoff() {
	next := time.Duration(float64(g.cur) * g.cfg.BackoffFactor)
	max := g.cfg.MaxInterval.Duration()
	if next > max {
		next = max
	}
	g.cur = next
}

func (g *Governor) recover() {
	next := time.Duration(float64(g.cur) * g.cfg.RecoverFactor)
	min := g.cfg.MinInterval.Duration()
	if next < min || next == 0 {
		next = min
	}
	g.cur = next
}

// Current returns the current interval.
func (g *Governor) Current() time.Duration { return g.cur }
