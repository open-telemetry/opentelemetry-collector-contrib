// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package consumerretry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"

import "time"

// Config defines configuration for retrying batches in case of receiving a retryable error from a downstream
// consumer. If the retryable error doesn't provide a delay, exponential backoff is applied.
type Config struct {
	// Enabled indicates whether to not retry sending logs in case of receiving a retryable error from a downstream
	// consumer. Default is false.
	Enabled bool `mapstructure:"enabled"`
	// InitialInterval the time to wait after the first failure before retrying. Default value is 1 second.
	InitialInterval time.Duration `mapstructure:"initial_interval"`
	// MaxInterval is the upper bound on backoff interval. Once this value is reached the delay between
	// consecutive retries will always be `MaxInterval`. Default value is 30 seconds.
	MaxInterval time.Duration `mapstructure:"max_interval"`
	// MaxElapsedTime is the maximum amount of time (including retries) spent trying to send a logs batch to
	// a downstream consumer. Once this value is reached, the data is discarded. It never stops if MaxElapsedTime == 0.
	// Default value is 5 minutes.
	MaxElapsedTime time.Duration `mapstructure:"max_elapsed_time"`
}

// NewDefaultConfig returns the default Config.
func NewDefaultConfig() Config {
	return Config{
		Enabled:         false,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		MaxElapsedTime:  5 * time.Minute,
	}
}
