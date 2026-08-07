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

var errNoValueProvided = errors.New("no value or file path provided")

// ValueResolver provides access to a secret value that may come from
// an inline config string or a watched file.
type ValueResolver interface {
	// Value returns the current secret value.
	Value() string
	// Start begins any background operations (e.g., file watching).
	// When retry options are supplied (see WithRetry), startup is retried
	// continuously until all prerequisites are available.
	Start(ctx context.Context, opts ...Option) error
	// Shutdown stops any background operations.
	Shutdown() error
}

// Option configures a ValueResolver.
type Option func(*options)

type options struct {
	onChange        func(string)
	retryEnabled    bool
	maxRetries      int
	initialInterval time.Duration
	retryInterval   time.Duration
}

// WithOnChange registers a callback invoked with the new value after each
// successful file reload. Not called for static values.
func WithOnChange(fn func(string)) Option {
	return func(o *options) { o.onChange = fn }
}

// WithRetry enables retrying startup until all prerequisites are available.
// The first retry is attempted after initialInterval; subsequent retries are
// spaced retryInterval apart. Startup gives up after maxRetries attempts.
func WithRetry(maxRetries int, initialInterval, retryInterval time.Duration) Option {
	return func(o *options) {
		o.retryEnabled = true
		o.maxRetries = maxRetries
		o.initialInterval = initialInterval
		o.retryInterval = retryInterval
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
		return newFileWatcher(filePath, logger, o.onChange), nil
	}
	if inlineValue == "" {
		return nil, errNoValueProvided
	}
	return staticValue(inlineValue), nil
}

// staticValue is a ValueResolver that returns a fixed string.
type staticValue string

func (s staticValue) Value() string                        { return string(s) }
func (staticValue) Start(context.Context, ...Option) error { return nil }
func (staticValue) Shutdown() error                        { return nil }
