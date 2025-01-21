// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver"

import (
	"context"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver/internal/metadata"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

const (
	defaultBroker            = "localhost:9092"
	defaultSessionTimeout    = 10 * time.Second
	defaultHeartbeatInterval = 3 * time.Second
)

// NewFactory should be called to create a factory with default values.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelBeta,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Brokers:           []string{defaultBroker},
		SessionTimeout:    defaultSessionTimeout,
		HeartbeatInterval: defaultHeartbeatInterval,
	}
}

func createExtension(
	_ context.Context,
	settings extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	config := cfg.(*Config)
	return newObserver(settings.Logger, config)
}
