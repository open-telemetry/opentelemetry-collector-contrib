// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedlogreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
)

// Config defines configuration for macOS Unified Logging receiver
type Config struct {
	fileconsumer.Config `mapstructure:",squash"`
	adapter.BaseConfig  `mapstructure:",squash"`

	// Encoding specifies the encoding extension to use for decoding traceV3 files.
	// This should reference an encoding extension ID (e.g., "macos_unified_logging_encoding").
	Encoding string `mapstructure:"encoding"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// getFileConsumerConfig returns a file consumer config with proper encoding settings
func (cfg *Config) getFileConsumerConfig() fileconsumer.Config {
	fcConfig := cfg.Config
	// Set encoding to "nop" since we'll handle decoding via the extension
	fcConfig.Encoding = "nop"
	return fcConfig
}

// Validate checks the receiver configuration is valid
func (cfg Config) Validate() error {
	if cfg.Encoding == "" {
		return errors.New("encoding is required for macOS Unified Logging receiver")
	}

	return nil
}

// unmarshalBody unmarshals the body of the receiver configuration
func (cfg *Config) unmarshalBody(component.Config) error {
	return nil
}
