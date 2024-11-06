// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/blobuploadprocessor"

import (
	"go.opentelemetry.io/collector/component"
)

// Config defines the overall configuration for this component.
type Config struct {
	Common       *CommonConfig `mapstructure:"common"`
	TracesConfig *TracesConfig `mapstructure:"traces"`
	LogsConfig   *LogsConfig   `mapstructure:"logs"`
}

// createTypedDefaultConfig instantiates a default, valid config.
func createTypedDefaultConfig() *Config {
	return &Config{}
}

// createDefaultConfig erases the type to allow use in component.
func createDefaultConfig() component.Config {
	return createTypedDefaultConfig()
}

// Validate ensures that the configuration is valid.
func (c *Config) Validate() error {
	// TODO: implement validation in subsequent PRs.
	return nil
}
