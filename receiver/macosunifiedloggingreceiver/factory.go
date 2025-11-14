// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedloggingreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
)

// NewFactory creates a factory for the macOS unified logging receiver
func NewFactory() receiver.Factory {
	return newFactoryAdapter()
}

// createDefaultConfig creates a config with default values
func createDefaultConfig() component.Config {
	// Default to live mode
	return &Config{
		MaxPollInterval: 30 * time.Second,
		MaxLogAge:       24 * time.Hour,
		Format:          "default",
	}
}
