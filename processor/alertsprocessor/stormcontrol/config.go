// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stormcontrol // import "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/stormcontrol"

import (
	"fmt"
	"time"
)

type Config struct {
	MinInterval             Duration `mapstructure:"min_interval"`   // e.g., "1s" or 1000000000
	MaxInterval             Duration `mapstructure:"max_interval"`   // e.g., "30s"
	BackoffFactor           float64  `mapstructure:"backoff_factor"` // >= 1.0
	RecoverFactor           float64  `mapstructure:"recover_factor"` // 0 < r <= 1.0
	MaxActiveAlerts         int      `mapstructure:"max_active_alerts"`
	MaxAlertsPerMinute      int      `mapstructure:"max_alerts_per_minute"`
	CircuitBreakerThreshold int      `mapstructure:"circuit_breaker_threshold"`
}

func (c Config) Validate() (Config, error) {
	minNS := c.MinInterval.Nanoseconds()
	maxNS := c.MaxInterval.Nanoseconds()

	if minNS <= 0 {
		return c, fmt.Errorf("stormcontrol.min_interval must be > 0, got %v", time.Duration(minNS))
	}
	if maxNS < minNS {
		return c, fmt.Errorf("stormcontrol.max_interval (%v) must be >= min_interval (%v)",
			time.Duration(maxNS), time.Duration(minNS))
	}
	if c.BackoffFactor < 1.0 {
		return c, fmt.Errorf("stormcontrol.backoff_factor must be >= 1.0, got %v", c.BackoffFactor)
	}
	if c.RecoverFactor <= 0.0 || c.RecoverFactor > 1.0 {
		return c, fmt.Errorf("stormcontrol.recover_factor must be in (0,1], got %v", c.RecoverFactor)
	}
	if c.MaxActiveAlerts < 0 || c.MaxAlertsPerMinute < 0 || c.CircuitBreakerThreshold < 0 {
		return c, fmt.Errorf("stormcontrol thresholds must be >= 0")
	}
	return c, nil
}

// Clamp d into [MinInterval, MaxInterval].
func (c Config) Clamp(d time.Duration) time.Duration {
	if d < c.MinInterval.Duration() {
		return c.MinInterval.Duration()
	}
	if d > c.MaxInterval.Duration() {
		return c.MaxInterval.Duration()
	}
	return d
}
