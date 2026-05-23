// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package credentialsfile provides a ValueResolver interface for resolving
// secret values from either inline config strings or watched files.
package credentialsfile // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile"

import (
	"context"
	"errors"

	"go.uber.org/zap"
)

var errNoValueProvided = errors.New("no value or file path provided")

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

// Option configures a ValueResolver.
type Option func(*options)

type options struct {
	onChange func(string)
}

// WithOnChange registers a callback invoked with the new value after each
// successful file reload. Not called for static values.
func WithOnChange(fn func(string)) Option {
	return func(o *options) { o.onChange = fn }
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

func (s staticValue) Value() string             { return string(s) }
func (staticValue) Start(context.Context) error { return nil }
func (staticValue) Shutdown() error             { return nil }
