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

	// TraceV3Paths specifies an alternate path to TraceV3 files. This can be a single file path or a glob pattern (e.g., "/path/to/logs/*.tracev3").
	TraceV3Paths []string `mapstructure:"tracev3_paths"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// getFileConsumerConfig returns a file consumer config with proper encoding settings
func (cfg *Config) getFileConsumerConfig() fileconsumer.Config {
	fcConfig := cfg.Config

	// If TraceV3Paths is specified, use it to override the include patterns
	if len(cfg.TraceV3Paths) > 0 {
		fcConfig.Include = cfg.TraceV3Paths
	}

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
		return errors.New("encoding_extension must be macosunifiedlogencoding for macOS Unified Logging receiver")
	}

	return nil
}
