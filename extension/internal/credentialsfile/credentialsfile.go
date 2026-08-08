// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package credentialsfile provides a ValueResolver interface for resolving
// secret values from either inline config strings or watched files.
package credentialsfile // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile"

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
)

var (
	errNoValueProvided                 = errors.New("no value or file path provided")
	errRetryOnFailureInvalidMaxRetries = errors.New("retry_on_failure.max_retries must be greater than 0 when retry_on_failure.enabled is true")
	errRetryOnFailureInvalidOffset     = errors.New("retry_on_failure.offset must be greater than 0 when retry_on_failure.enabled is true")
)

// ValueResolver provides access to a secret value that may come from
// an inline config string or a watched file.
type ValueResolver interface {
	// Value returns the current secret value.
	Value() string
	// Start begins any background operations (e.g., file watching).
	Start(ctx context.Context) error
	// Shutdown stops any background operations.
	Shutdown() error
}

// RetryOnFailureConfig configures retry on missing credentials file.
type RetryOnFailureConfig struct {
	// Enabled defines if any retry logic should be done on a missing file.
	// Defaults to false.
	Enabled bool `mapstructure:"enabled,omitempty"`

	// MaxRetries is the maximum number of times to retry reading the file.
	MaxRetries int `mapstructure:"max_retries,omitempty"`

	// Offset is the interval between retries.
	Offset time.Duration `mapstructure:"offset,omitempty"`
}

func (rfg *RetryOnFailureConfig) Validate() error {
	if rfg.Enabled {
		if rfg.MaxRetries <= 0 {
			return errRetryOnFailureInvalidMaxRetries
		}
		if rfg.Offset <= 0 {
			return errRetryOnFailureInvalidOffset
		}
	}
	return nil
}

// Option configures a ValueResolver.
type Option func(*options)

type options struct {
	onChange func(string)
	retryCfg RetryOnFailureConfig
}

// WithOnChange registers a callback invoked with the new value after each
// successful file reload. Not called for static values.
func WithOnChange(fn func(string)) Option {
	return func(o *options) { o.onChange = fn }
}

// WithRetry enables retrying to read a credentials file.
func WithRetry(rfc RetryOnFailureConfig) Option {
	return func(o *options) {
		o.retryCfg = rfc
	}
}

// NewValueResolver returns a ValueResolver appropriate for the given inputs.
// If filePath is non-empty, returns a FileWatcher that watches the file for changes.
// Otherwise returns a StaticValue wrapping inlineValue.
// Returns an error if both inlineValue and filePath are empty.
func NewValueResolver(inlineValue, filePath string, logger *zap.Logger, opts ...Option) (ValueResolver, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if filePath != "" {
		return newFileWatcher(filePath, logger, o.onChange, o.retryCfg), nil
	}
	if inlineValue == "" {
		return nil, errNoValueProvided
	}
	return staticValue(inlineValue), nil
}

// staticValue is a ValueResolver that returns a fixed string.
type staticValue string

func (s staticValue) Value() string             { return string(s) }
func (staticValue) Start(context.Context) error { return nil }
func (staticValue) Shutdown() error             { return nil }
