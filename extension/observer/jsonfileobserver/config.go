// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonfileobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/jsonfileobserver"

import (
	"errors"
	"time"
)

// Config defines configuration for the JSON file observer extension.
type Config struct {
	// Path is the path to the JSON file to observe.
	// The file should contain an array of endpoint objects.
	Path string `mapstructure:"path"`
	// RefreshInterval is how often to check for file changes.
	// Default is 10 seconds.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Path == "" {
		return errors.New("path must be specified")
	}
	if cfg.RefreshInterval < 0 {
		return errors.New("refresh_interval must be non-negative")
	}
	return nil
}
