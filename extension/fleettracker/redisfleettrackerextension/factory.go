// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisfleettrackerextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/fleettracker/redisfleettrackerextension"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/fleettracker/redisfleettrackerextension/internal/metadata"
)

// NewFactory creates the redis_fleet_tracker extension factory.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		create,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		KeyPrefix:         "fleet_tracker",
		HeartbeatInterval: 3 * time.Second,
		TTL:               10 * time.Second,
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
}

func create(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	oCfg := cfg.(*Config)
	if oCfg.Fleet == "" {
		oCfg.Fleet = set.ID.String()
	}

	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	memberID := computeMemberID(set)

	return newExtension(oCfg, set.Logger, telemetryBuilder, memberID), nil
}
