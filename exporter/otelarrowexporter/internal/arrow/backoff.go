// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow

import (
	"github.com/cenkalti/backoff/v7"
	"go.opentelemetry.io/collector/config/configretry"
)

// NewDefaultStreamRestartConfig uses the standard retry configuration with no elapsed-time limit.
func NewDefaultStreamRestartConfig() configretry.BackOffConfig {
	cfg := configretry.NewDefaultBackOffConfig()
	cfg.MaxElapsedTime = 0
	return cfg
}

func newStreamRestartBackOff(cfg configretry.BackOffConfig) *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = cfg.InitialInterval
	b.RandomizationFactor = cfg.RandomizationFactor
	b.Multiplier = cfg.Multiplier
	b.MaxInterval = cfg.MaxInterval
	b.Reset()
	return b
}
