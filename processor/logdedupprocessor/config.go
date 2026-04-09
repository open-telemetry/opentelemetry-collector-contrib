// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package logdedupprocessor provides a processor that counts logs as metrics.
package logdedupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
)

// TimestampMode defines how the timestamp of the emitted log record is set.
type TimestampMode string

const (
	// TimestampModeObserved sets the log record timestamp to when the aggregated log was emitted.
	TimestampModeObserved TimestampMode = "observed"
	// TimestampModePreserved preserves the log record timestamp from the first log received in the aggregation window.
	TimestampModePreserved TimestampMode = "preserved"
)

// Config defaults
const (
	// defaultInterval is the default export interval.
	defaultInterval = 10 * time.Second

	// defaultLogCountAttribute is the default log count attribute
	defaultLogCountAttribute = "log_count"

	// defaultTimezone is the default timezone
	defaultTimezone = "UTC"

	// defaultTimestampMode is the default timestamp mode
	defaultTimestampMode = TimestampModeObserved

	// bodyField is the name of the body field
	bodyField = "body"

	// attributeField is the name of the attribute field
	attributeField = "attributes"
)

// Config errors
var (
	errInvalidLogCountAttribute = errors.New("log_count_attribute must be set")
	errInvalidInterval          = errors.New("interval must be greater than 0")
	errCannotExcludeBody        = errors.New("cannot exclude the entire body")
	errCannotIncludeBody        = errors.New("cannot include the entire body")
	errInvalidTimestampMode     = errors.New("timestamp_mode must be one of 'observed' or 'preserved'")
)

// Config is the config of the processor.
type Config struct {
	LogCountAttribute string        `mapstructure:"log_count_attribute"`
	Interval          time.Duration `mapstructure:"interval"`
	Timezone          string        `mapstructure:"timezone"`
	ExcludeFields     []string      `mapstructure:"exclude_fields"`
	IncludeFields     []string      `mapstructure:"include_fields"`
	Conditions        []string      `mapstructure:"conditions"`
	TimestampMode     TimestampMode `mapstructure:"timestamp_mode"`
}

// createDefaultConfig returns the default config for the processor.
func createDefaultConfig() component.Config {
	return &Config{
		LogCountAttribute: defaultLogCountAttribute,
		Interval:          defaultInterval,
		Timezone:          defaultTimezone,
		TimestampMode:     defaultTimestampMode,
		ExcludeFields:     []string{},
		IncludeFields:     []string{},
		Conditions:        []string{},
	}
}

// Validate validates the configuration
func (c Config) Validate() error {
	if c.Interval <= 0 {
		return errInvalidInterval
	}

	if c.LogCountAttribute == "" {
		return errInvalidLogCountAttribute
	}

	_, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return fmt.Errorf("timezone is invalid: %w", err)
	}

	if len(c.ExcludeFields) > 0 && len(c.IncludeFields) > 0 {
		return errors.New("cannot define both exclude_fields and include_fields")
	}

	err = c.validateExcludeFields()
	if err != nil {
		return err
	}

	err = c.validateIncludeFields()
	if err != nil {
		return err
	}

	switch c.TimestampMode {
	case TimestampModeObserved, TimestampModePreserved, "":
	default:
		return errInvalidTimestampMode
	}

	return nil
}

// validateExcludeFields validates that all the exclude fields
func (c Config) validateExcludeFields() error {
	knownExcludeFields := make(map[string]struct{})

	for _, field := range c.ExcludeFields {
		// Special check to make sure the entire body is not excluded
		if field == bodyField {
			return errCannotExcludeBody
		}

		// Split and ensure the field starts with `body` or `attributes`
		parts := strings.Split(field, fieldDelimiter)
		if parts[0] != bodyField && parts[0] != attributeField {
			return fmt.Errorf("an excludefield must start with %s or %s", bodyField, attributeField)
		}

		// If a field is valid make sure we haven't already seen it
		if _, ok := knownExcludeFields[field]; ok {
			return fmt.Errorf("duplicate exclude_field %s", field)
		}

		knownExcludeFields[field] = struct{}{}
	}

	return nil
}

// validateIncludeFields validates that all the exclude fields
func (c Config) validateIncludeFields() error {
	knownFields := make(map[string]struct{})

	for _, field := range c.IncludeFields {
		// Special check to make sure the entire body is not included
		if field == bodyField {
			return errCannotIncludeBody
		}

		// Split and ensure the field starts with `body` or `attributes`
		parts := strings.Split(field, fieldDelimiter)
		if parts[0] != bodyField && parts[0] != attributeField {
			return fmt.Errorf("an include_fields must start with %s or %s", bodyField, attributeField)
		}

		// If a field is valid make sure we haven't already seen it
		if _, ok := knownFields[field]; ok {
			return fmt.Errorf("duplicate include_fields %s", field)
		}

		knownFields[field] = struct{}{}
	}

	return nil
}
