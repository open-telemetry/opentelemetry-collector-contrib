// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceexhaustedretryextension

import (
	"errors"
	"time"
)

// Config holds configuration for the resourceexhaustedretryextension.
type Config struct {
	// RetryDelay is the base duration advertised in RetryInfo (gRPC) or Retry-After (HTTP).
	// Zero (default) disables injection — the extension is a complete no-op.
	RetryDelay time.Duration `mapstructure:"retry_delay"`

	// Jitter is the maximum additional random duration added to RetryDelay per response.
	// Each response independently samples: effective_delay = RetryDelay + rand[0, Jitter].
	// Zero means no jitter.
	Jitter time.Duration `mapstructure:"jitter"`
}

func (c *Config) Validate() error {
	if c.RetryDelay < 0 {
		return errors.New("retry_delay must be non-negative")
	}
	if c.Jitter < 0 {
		return errors.New("jitter must be non-negative")
	}
	return nil
}
