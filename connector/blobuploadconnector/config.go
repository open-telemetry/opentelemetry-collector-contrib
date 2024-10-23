// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadconnector

// Config defines the overall configuration for this component.
type Config struct {
	Common       *CommonConfig `mapstructure:"common"`
	TracesConfig *TracesConfig `mapstructure:"traces"`
	LogsConfig   *LogsConfig   `mapstructure:"logs"`
}

// NewDefaultConfig instantiates a default, valid config.
func NewDefaultConfig() *Config {
	return &Config{}
}

// Validate ensures that the configuration is valid.
func (c *Config) Validate() error {
	// TODO: implement validation in subsequent PRs.
	return nil
}
