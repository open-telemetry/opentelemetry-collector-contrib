// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
//go:generate make mdatagen

package sumologicextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension"

import (
	"context"

	"github.com/cenkalti/backoff/v7"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/internal/credentials"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/internal/metadata"
)

const (
	// The value of extension "type" in configuration.
	DefaultAPIBaseURL = "https://open-collectors.sumologic.com"
)

// NewFactory creates a factory for Sumo Logic extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	defaultCredsPath, err := credentials.GetDefaultCollectorCredentialsDirectory()
	if err != nil {
		return nil
	}

	clientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfig.MaxIdleConns = 0
	clientConfig.IdleConnTimeout = 0
	clientConfig.ForceAttemptHTTP2 = false
	return &Config{
		ClientConfig:                  clientConfig,
		APIBaseURL:                    DefaultAPIBaseURL,
		HeartBeatInterval:             DefaultHeartbeatInterval,
		CollectorCredentialsDirectory: defaultCredsPath,
		Clobber:                       false,
		DiscoverCollectorTags:         true,
		ForceRegistration:             false,
		Ephemeral:                     false,
		TimeZone:                      "",
		StickySessionEnabled:          false,
		UpdateMetadata:                true,
		BackOff: backOffConfig{
			InitialInterval: backoff.DefaultInitialInterval,
			MaxInterval:     backoff.DefaultMaxInterval,
			MaxElapsedTime:  backoff.DefaultMaxElapsedTime,
		},
	}
}

func createExtension(_ context.Context, params extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)
	return newSumologicExtension(config, params.Logger, params.ID, params.BuildInfo.Version)
}
