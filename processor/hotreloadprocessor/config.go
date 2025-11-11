// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotreloadprocessor

import (
	"fmt"
	"time"
)

// Config defines configuration for Resource processor.
type Config struct {
	// ConfigurationPrefix is the prefix to the s3 configuration file.
	ConfigurationPrefix string `mapstructure:"configuration_prefix"`
	// EncryptionKey is the key used to encrypt the configuration file.
	EncryptionKey string `mapstructure:"encryption_key"`
	// Region is the region of the S3 bucket where the configuration file is stored.
	Region string `mapstructure:"region"`
	// RefreshInterval is the interval at which the configuration is refreshed.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	// ShutdownDelay is the delay before shutting down the old subprocessors.
	ShutdownDelay time.Duration `mapstructure:"shutdown_delay"`
	// WatchPath is the path to the file that is watched for changes.
	WatchPath *string `mapstructure:"watch_path"`
}

func (cfg *Config) Validate() error {
	if cfg.ConfigurationPrefix == "" {
		return fmt.Errorf("configuration_prefix must be specified")
	}

	if cfg.EncryptionKey == "" {
		return fmt.Errorf("encryption_key must be specified")
	}

	if cfg.Region == "" {
		return fmt.Errorf("region must be specified")
	}

	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = 60 * time.Second
	}

	if cfg.ShutdownDelay <= 0 {
		cfg.ShutdownDelay = 10 * time.Second
	}

	if cfg.RefreshInterval <= cfg.ShutdownDelay {
		return fmt.Errorf("refresh_interval must be greater than shutdown_delay")
	}

	return nil
}
