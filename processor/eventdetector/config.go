// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eventdetector // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/eventdetector"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

// LogRule opens/refreshes a window when any log record satisfies all
// conditions (OTTL log context).
type LogRule struct {
	Name       string   `mapstructure:"name"`
	Window     string   `mapstructure:"window"`
	Conditions []string `mapstructure:"conditions"`
	// Ttl bounds the boost period. Required.
	Ttl     time.Duration `mapstructure:"ttl"`
	// Cooldown prevents re-opening for this long after expiry, damping
	// flappy windows. Defaults to Ttl/2.
	Cooldown time.Duration `mapstructure:"cooldown"`
}

// MetricRule opens/refreshes a window when any metric datapoint satisfies
// all conditions (OTTL datapoint context).
type MetricRule struct {
	Name       string   `mapstructure:"name"`
	Window     string   `mapstructure:"window"`
	Conditions []string `mapstructure:"conditions"`
	Ttl        time.Duration `mapstructure:"ttl"`
	Cooldown   time.Duration `mapstructure:"cooldown"`
}

type Config struct {
	// SignalstateExtension is the component ID of the installed
	// extension/signalstate the detector writes windows into. Required.
	SignalstateExtension *component.ID `mapstructure:"signalstate_extension"`

	LogRules    []LogRule   `mapstructure:"log_rules"`
	MetricRules []MetricRule `mapstructure:"metric_rules"`
}

var _ confmap.Validator = (*Config)(nil)

func (cfg *Config) Validate() error {
	if cfg.SignalstateExtension == nil {
		return fmt.Errorf("signalstate_extension must be set")
	}
	seen := map[string]struct{}{}
	for i, r := range cfg.LogRules {
		if r.Name == "" {
			return fmt.Errorf("log_rules[%d]: name must be set", i)
		}
		if _, dup := seen["log:"+r.Name]; dup {
			return fmt.Errorf("log_rules[%d]: duplicate name %q", i, r.Name)
		}
		seen["log:"+r.Name] = struct{}{}
		if r.Window == "" {
			return fmt.Errorf("log_rules[%d] %q: window must be set", i, r.Name)
		}
		if r.Ttl <= 0 {
			return fmt.Errorf("log_rules[%d] %q: ttl must be positive", i, r.Name)
		}
	}
	for i, r := range cfg.MetricRules {
		if r.Name == "" {
			return fmt.Errorf("metric_rules[%d]: name must be set", i)
		}
		if _, dup := seen["metric:"+r.Name]; dup {
			return fmt.Errorf("metric_rules[%d]: duplicate name %q", i, r.Name)
		}
		seen["metric:"+r.Name] = struct{}{}
		if r.Window == "" {
			return fmt.Errorf("metric_rules[%d] %q: window must be set", i, r.Name)
		}
		if r.Ttl <= 0 {
			return fmt.Errorf("metric_rules[%d] %q: ttl must be positive", i, r.Name)
		}
	}
	return nil
}

// effectiveCooldown is the detector-level hysteresis: once a window closes,
// it may not be re-opened for this long.
func (r LogRule) effectiveCooldown() time.Duration {
	if r.Cooldown > 0 {
		return r.Cooldown
	}
	return r.Ttl / 2
}

func (r MetricRule) effectiveCooldown() time.Duration {
	if r.Cooldown > 0 {
		return r.Cooldown
	}
	return r.Ttl / 2
}
