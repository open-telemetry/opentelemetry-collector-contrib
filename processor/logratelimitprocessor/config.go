// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logratelimitprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logratelimitprocessor"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defaults
const (
	// defaultAllowedLogRate is default allowed logs per configured interval
	defaultAllowedLogRate = 30000

	// defaultInterval is default time interval for applying rate-limit after allowed logs
	defaultInterval = 60 * time.Second

	// bodyField is the name of the body field
	bodyField = "body"

	// attributeField is the name of the attribute field
	attributeField = "attributes"

	// resourceField is the name of the resource field
	resourceField = "resource"
)

// Config errors
var (
	errInvalidAllowedRate = errors.New("allowed_rate must be greater than 0")
	errInvalidInterval    = errors.New("interval must be greater than 0")
)

// Config is the config of the processor
type Config struct {
	Conditions      []string      `mapstructure:"conditions"`
	AllowedRate     uint64        `mapstructure:"allowed_rate"`
	Interval        time.Duration `mapstructure:"interval"`
	RateLimitFields []string      `mapstructure:"rate_limit_fields"`
}

// createDefaultConfig returns the default config for the processor.
func createDefaultConfig() component.Config {
	return &Config{
		Conditions:      []string{},
		AllowedRate:     defaultAllowedLogRate,
		Interval:        defaultInterval,
		RateLimitFields: []string{},
	}
}

// Validate validates the configuration
func (c Config) Validate() error {
	if c.Interval <= 0 {
		return errInvalidInterval
	}

	if c.AllowedRate < 0 {
		return errInvalidAllowedRate
	}

	return c.validateRateLimitFields()
}

// validateRateLimitFields validates the rate_limit_fields
func (c Config) validateRateLimitFields() error {
	knownExcludeFields := make(map[string]struct{})

	for _, field := range c.RateLimitFields {
		// Split and ensure the field starts with `body` or `attributes` or resource
		parts := strings.Split(field, fieldDelimiter)
		if parts[0] != bodyField && parts[0] != attributeField && parts[0] != resourceField {
			return fmt.Errorf("an ratelimit field must start with %s or %s or %s", bodyField, attributeField, resourceField)
		}

		// If a field is valid make sure we haven't already seen it
		if _, ok := knownExcludeFields[field]; ok {
			return fmt.Errorf("duplicate rate_limit_field %s", field)
		}

		knownExcludeFields[field] = struct{}{}
	}

	return nil
}
