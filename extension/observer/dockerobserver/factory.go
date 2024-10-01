// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/dockerobserver"

import (
	"context"
	"time"

	"github.com/docker/docker/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/dockerobserver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
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
		Config: docker.Config{
			Endpoint:         client.DefaultDockerHost,
			Timeout:          5 * time.Second,
			DockerAPIVersion: defaultDockerAPIVersion,
		},
		CacheSyncInterval: 60 * time.Minute,
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
