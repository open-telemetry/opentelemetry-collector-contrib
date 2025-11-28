// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/metadata"
)

// createValidTestConfig creates a config suitable for testing
func createValidTestConfig() *Config {
	return &Config{
		ServerConfig: configgrpc.NewDefaultServerConfig(),
		YANG: YANGConfig{
			EnableRFCParser: true,
			CacheModules:    true,
			MaxModules:      1000,
		},
	}
}

// createTestReceiver creates a receiver with valid test configuration
func createTestReceiver() (*yangReceiver, error) {
	config := createValidTestConfig()

	// Create proper receiver settings
	settings := receiver.Settings{
		ID:                component.NewID(metadata.Type),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}

	// Create a test metrics consumer
	consumer := consumertest.NewNop()

	rcvr, err := createMetricsReceiver(context.Background(), settings, config, consumer)
	yangRcvr := rcvr.(*yangReceiver)
	return yangRcvr, err
}

// createTestSettings creates proper receiver settings for testing
func createTestSettings() receiver.Settings {
	return receiver.Settings{
		ID:                component.NewID(metadata.Type),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}
}
