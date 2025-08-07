// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedlogreceiver"

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
)

// Config defines configuration for the macOS Unified Logging receiver
type Config struct {
	fileconsumer.Config `mapstructure:",squash"`
	adapter.BaseConfig  `mapstructure:",squash"`

	// Encoding specifies the encoding extension to use for decoding traceV3 files.
	// This should reference an encoding extension ID (e.g., "macos_unified_logging_encoding").
	Encoding string `mapstructure:"encoding_extension"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// getFileConsumerConfig returns a file consumer config with proper encoding settings
func (cfg *Config) getFileConsumerConfig() fileconsumer.Config {
	fcConfig := cfg.Config
	// Set encoding to "nop" since we'll handle decoding via the extension
	fcConfig.Encoding = "nop"

	// Enable file path attributes so we can access the file paths in the consume function
	fcConfig.IncludeFilePath = true
	fcConfig.IncludeFileName = true

	return fcConfig
}

// Validate checks the receiver configuration is valid
func (cfg Config) Validate() error {
	if cfg.Encoding != "macosunifiedlogencoding" {
		return errors.New("encoding must be macosunifiedlogencoding for macOS Unified Logging receiver")
	}

	return nil
}
