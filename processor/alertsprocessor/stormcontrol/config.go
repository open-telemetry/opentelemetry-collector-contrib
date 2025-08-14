package stormcontrol

import (
	"fmt"
	"time"
)

type Config struct {
	// Hard caps used by the governor when adapting the evaluation ticker.
	MinInterval Duration `mapstructure:"min_interval"` // e.g., "1s"
	MaxInterval Duration `mapstructure:"max_interval"` // e.g., "30s"`

	// Backoff when storms are detected; Recover when storms subside.
	BackoffFactor float64 `mapstructure:"backoff_factor"` // >= 1.0
	RecoverFactor float64 `mapstructure:"recover_factor"` // 0 < r <= 1.0 (move toward MinInterval)

	// Thresholds for deciding stormy conditions.
	MaxActiveAlerts         int `mapstructure:"max_active_alerts"`         // e.g., 1000
	MaxAlertsPerMinute      int `mapstructure:"max_alerts_per_minute"`     // e.g., 2000
	CircuitBreakerThreshold int `mapstructure:"circuit_breaker_threshold"` // optional: trip hard stop
}

// Validate performs semantic checks and returns a canonicalized copy.
func (c Config) Validate() (Config, error) {
	min := c.MinInterval.Duration()
	max := c.MaxInterval.Duration()

	if min <= 0 {
		return c, fmt.Errorf("stormcontrol.min_interval must be > 0, got %v", min)
	}
	if max < min {
		return c, fmt.Errorf("stormcontrol.max_interval (%v) must be >= min_interval (%v)", max, min)
	}
	if c.BackoffFactor < 1.0 {
		return c, fmt.Errorf("stormcontrol.backoff_factor must be >= 1.0, got %v", c.BackoffFactor)
	}
	if c.RecoverFactor <= 0.0 || c.RecoverFactor > 1.0 {
		return c, fmt.Errorf("stormcontrol.recover_factor must be in (0,1], got %v", c.RecoverFactor)
	}
	if c.MaxActiveAlerts < 0 || c.MaxAlertsPerMinute < 0 {
		return c, fmt.Errorf("stormcontrol thresholds must be >= 0")
	}
	if c.CircuitBreakerThreshold < 0 {
		return c, fmt.Errorf("stormcontrol.circuit_breaker_threshold must be >= 0")
	}
	return c, nil
}

// Clamp clamps d into [MinInterval, MaxInterval].
func (c Config) Clamp(d time.Duration) time.Duration {
	if d < c.MinInterval.Duration() {
		return c.MinInterval.Duration()
	}
	if d > c.MaxInterval.Duration() {
		return c.MaxInterval.Duration()
	}
	return d
}
