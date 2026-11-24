// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deadband // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deadband"

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)


// AdaptiveConfig wires the DB v2 boost path: while the named signalstate
// window is open, suppression is suspended (all gauges pass through, state
// still updates so calm-state resumes from the newest sent values) and the
// previously suppressed series are flushed on boost entry. Fail toward
// visibility: an unreachable extension behaves as calm state.
type AdaptiveConfig struct {
	Enabled              bool         `mapstructure:"enabled"`
	SignalstateExtension *component.ID `mapstructure:"signalstate_extension"`
	// Window is the signalstate window name that triggers the boost.
	Window string `mapstructure:"window"`
}

func (a *AdaptiveConfig) Validate() error {
	if !a.Enabled {
		return nil
	}
	if a.SignalstateExtension == nil {
		return fmt.Errorf("adaptive.signalstate_extension must be set when adaptive is enabled")
	}
	if a.Window == "" {
		return fmt.Errorf("adaptive.window must be set when adaptive is enabled")
	}
	return nil
}

// boostClient is the surface deadband reads from the signalstate extension.
type boostClient interface {
	IsOpen(ctx context.Context, name string) bool
	ExpiresAt(ctx context.Context, name string) time.Time
}

// Mode selects the deadband comparison function.
type Mode string

const (
	ModeAbsolute Mode = "absolute"
	ModePercent  Mode = "percent"
)

type Match struct {
	// MetricNames selects by exact metric name unless MatchType is regexp.
	MetricNames []string `mapstructure:"metric_names"`
	// MatchType is "strict" (default) or "regexp".
	MatchType string `mapstructure:"match_type"`

	re regExpSet
}

type regExpSet struct {
	re []*regexp.Regexp
}

func (m *Match) init() error {
	switch m.MatchType {
	case "", "strict":
		m.MatchType = "strict"
	case "regexp":
		for _, n := range m.MetricNames {
			re, err := regexp.Compile(n)
			if err != nil {
				return fmt.Errorf("invalid metric_names regexp %q: %w", n, err)
			}
			m.re.re = append(m.re.re, re)
		}
	default:
		return fmt.Errorf("match_type %q is not one of: strict, regexp", m.MatchType)
	}
	return nil
}

func (m *Match) matches(name string) bool {
	if m == nil || len(m.MetricNames) == 0 {
		return true // no match configured: apply to every metric
	}
	switch m.MatchType {
	case "regexp":
		for _, re := range m.re.re {
			if re.MatchString(name) {
				return true
			}
		}
		return false
	default:
		for _, n := range m.MetricNames {
			if n == name {
				return true
			}
		}
		return false
	}
}

type StateConfig struct {
	// MaxSeries is the LRU cardinality cap. series beyond the cap pass
	// through unsuppressed (fail toward visibility).
	MaxSeries int `mapstructure:"max_series"`
}

type Config struct {
	Match Match `mapstructure:"match"`

	// Mode is "absolute" or "percent".
	Mode Mode `mapstructure:"mode"`
	// Threshold is the deadband width (absolute units, or a fraction for
	// percent mode).
	Threshold float64 `mapstructure:"threshold"`
	// MinReference guards percent mode against divide-by-noise near zero.
	// Defaults to the threshold itself.
	MinReference float64 `mapstructure:"min_reference"`

	// StaleAfter bounds silence: after this long without emission the
	// last-sent value is restated (heartbeat). Zero means unbounded and
	// must be explicit. v1 default: 5m.
	StaleAfter time.Duration `mapstructure:"stale_after"`

	// RepeatAttribute tags heartbeat records. Defaults to
	// "telemetry.deadband.repeated".
	RepeatAttribute string `mapstructure:"repeat_attribute"`

	State    StateConfig    `mapstructure:"state"`
	Adaptive AdaptiveConfig `mapstructure:"adaptive"`
}

var _ confmap.Validator = (*Config)(nil)

func (cfg *Config) Validate() error {
	if err := cfg.Match.init(); err != nil {
		return err
	}
	if err := cfg.Adaptive.Validate(); err != nil {
		return err
	}
	switch cfg.Mode {
	case "":
		cfg.Mode = ModeAbsolute
	case ModeAbsolute, ModePercent:
	default:
		return fmt.Errorf("mode %q is not one of: absolute, percent", cfg.Mode)
	}
	if cfg.Threshold <= 0 {
		return fmt.Errorf("threshold must be positive")
	}
	if cfg.MinReference < 0 {
		return fmt.Errorf("min_reference must not be negative")
	}
	switch cfg.StaleAfter {
	case 0:
		// explicit unbounded: allowed
	default:
		if cfg.StaleAfter < 0 {
			return fmt.Errorf("stale_after must not be negative")
		}
	}
	return nil
}

// sortedNames returns metric names in stable order (validation error text).
func sortedNames(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var _ = sortedNames // keep sorting helper for v2 group mode diagnostics
