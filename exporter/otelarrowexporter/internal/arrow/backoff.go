// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow

import (
	"errors"
	"math"
	"time"

	"github.com/cenkalti/backoff/v7"
	"go.opentelemetry.io/collector/config/configretry"
)

// StreamRestartConfig controls the delay between failed stream attempts.
// Stream retries continue until shutdown or downgrade, without an elapsed-time limit.
type StreamRestartConfig struct {
	InitialInterval     time.Duration `mapstructure:"initial_interval"`
	RandomizationFactor float64       `mapstructure:"randomization_factor"`
	Multiplier          float64       `mapstructure:"multiplier"`
	MaxInterval         time.Duration `mapstructure:"max_interval"`
}

// NewDefaultStreamRestartConfig uses the same interval defaults as batch retries.
func NewDefaultStreamRestartConfig() StreamRestartConfig {
	cfg := configretry.NewDefaultBackOffConfig()
	return StreamRestartConfig{
		InitialInterval:     cfg.InitialInterval,
		RandomizationFactor: cfg.RandomizationFactor,
		Multiplier:          cfg.Multiplier,
		MaxInterval:         cfg.MaxInterval,
	}
}

// Validate prevents configurations that can produce an immediate retry loop.
func (cfg StreamRestartConfig) Validate() error {
	if cfg.InitialInterval <= 0 {
		return errors.New("initial_interval must be > 0")
	}
	if cfg.MaxInterval < cfg.InitialInterval {
		return errors.New("max_interval must be >= initial_interval")
	}
	if math.IsNaN(cfg.RandomizationFactor) || cfg.RandomizationFactor < 0 || cfg.RandomizationFactor > 1 {
		return errors.New("randomization_factor must be between 0 and 1")
	}
	if math.IsNaN(cfg.Multiplier) || math.IsInf(cfg.Multiplier, 0) || cfg.Multiplier < 1 {
		return errors.New("multiplier must be finite and >= 1")
	}
	return nil
}

func (cfg StreamRestartConfig) newBackOff() *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = cfg.InitialInterval
	b.RandomizationFactor = cfg.RandomizationFactor
	b.Multiplier = cfg.Multiplier
	b.MaxInterval = cfg.MaxInterval
	b.Reset()
	return b
}
