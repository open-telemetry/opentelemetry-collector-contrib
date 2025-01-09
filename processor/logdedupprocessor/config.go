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

// Config defaults
const (
	// defaultInterval is the default export interval.
	defaultInterval = 10 * time.Second

	// defaultLogCountAttribute is the default log count attribute
	defaultLogCountAttribute = "log_count"

	// defaultTimezone is the default timezone
	defaultTimezone = "UTC"

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
)

// Config is the config of the processor.
type Config struct {
	LogCountAttribute string        `mapstructure:"log_count_attribute"`
	Interval          time.Duration `mapstructure:"interval"`
	Timezone          string        `mapstructure:"timezone"`
	ExcludeFields     []string      `mapstructure:"exclude_fields"`
	Conditions        []string      `mapstructure:"conditions"`
	Key               string        `mapstructure:"key"`
}

// createDefaultConfig returns the default config for the processor.
func createDefaultConfig() component.Config {
	return &Config{
		LogCountAttribute: defaultLogCountAttribute,
		Interval:          defaultInterval,
		Timezone:          defaultTimezone,
		ExcludeFields:     []string{},
		Conditions:        []string{},
		Key:               "",
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

	if err = c.validateExcludeFields(); err != nil {
		return err
	}

	if err = c.validateKey(); err != nil {
		return err
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

// validateKey validates that key is a valid field.
// Valid fields have the format of `body.field` or `attributes.field`.
func (c Config) validateKey() error {
	if c.Key == "" {
		return nil
	}

	// Special check to make sure the entire body or attributes is not used as key
	switch c.Key {
	case bodyField, attributeField:
		return fmt.Errorf("cannot use the entire %s as key", c.Key)
	}

	// Split and ensure the field starts with `body` or `attributes`
	parts := strings.Split(c.Key, fieldDelimiter)
	if parts[0] != bodyField && parts[0] != attributeField {
		return fmt.Errorf("a key must start with %s or %s", bodyField, attributeField)
	}

	return nil
}
