package stormcontrol // import "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/stormcontrol"

import (
	"math"
	"time"
)

// Governor adapts the processor's evaluation interval inside [MinInterval, MaxInterval].
// All internal math uses integer nanoseconds; public getters return time.Duration.
type Governor struct {
	cfg    Config
	curNS  int64   // current interval in ns
	emaAPM float64 // exponential moving average of alerts-per-minute
	alpha  float64 // [0,1], smoothing factor for EMA
}

// New expects cfg to be validated.
func New(cfg Config) *Governor {
	return &Governor{
		cfg:   cfg,
		curNS: cfg.MinInterval.Nanoseconds(),
		alpha: 0.2,
	}
}

// Update adapts the interval based on currently active instances and APM.
// Returns the next interval as time.Duration (but stored internally in ns).
func (g *Governor) Update(active int, apm float64) time.Duration {
	// Smooth APM.
	g.emaAPM = g.alpha*apm + (1.0-g.alpha)*g.emaAPM

	stormy := (g.cfg.MaxActiveAlerts > 0 && active > g.cfg.MaxActiveAlerts) ||
		(g.cfg.MaxAlertsPerMinute > 0 && int(g.emaAPM) > g.cfg.MaxAlertsPerMinute)

	if stormy {
		g.backoff()
	} else {
		g.recover()
	}
	return time.Duration(g.curNS)
}

// Current returns the current interval.
func (g *Governor) Current() time.Duration { return time.Duration(g.curNS) }

func (g *Governor) backoff() {
	next := int64(math.Round(float64(g.curNS) * g.cfg.BackoffFactor))
	max := g.cfg.MaxInterval.Nanoseconds()
	if next > max {
		next = max
	}
	g.curNS = next
}

func (g *Governor) recover() {
	next := int64(math.Round(float64(g.curNS) * g.cfg.RecoverFactor))
	min := g.cfg.MinInterval.Nanoseconds()
	if next < min || next == 0 {
		next = min
	}
	g.curNS = next
}
