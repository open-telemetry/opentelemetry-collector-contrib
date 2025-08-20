// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package throttlingprocessor provides a processor that provides throttling for logs.
package throttlingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/throttlingprocessor"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config errors
var (
	errInvalidInterval      = errors.New("interval must be defined and greater than 0")
	errInvalidThreshold     = errors.New("threshold must be defined and greater than 0")
	errInvalidKeyExpression = errors.New("key_expression must be defined")
)

// Config is the config of the processor.
type Config struct {
	Interval      time.Duration `mapstructure:"interval"`
	Threshold     int64         `mapstructure:"threshold"`
	KeyExpression string        `mapstructure:"key_expression"`
	Conditions    []string      `mapstructure:"conditions"`
}

// createDefaultConfig returns the default config for the processor.
func createDefaultConfig() component.Config {
	return &Config{
		Conditions: []string{},
	}
}

// Validate validates the configuration
func (c Config) Validate() error {
	if c.Interval <= 0 {
		return errInvalidInterval
	}
	if c.Threshold <= 0 {
		return errInvalidThreshold
	}
	if c.KeyExpression == "" {
		return errInvalidKeyExpression
	}
	return nil
}
