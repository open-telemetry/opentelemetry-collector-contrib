// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
//go:generate mdatagen metadata.yaml

package sumologicextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension"

import (
	"context"

	"github.com/cenkalti/backoff/v4"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

const (
	// The value of extension "type" in configuration.
	typeStr           = "sumologic"
	DefaultApiBaseUrl = "https://open-collectors.sumologic.com"
)

// NewFactory creates a factory for Sumo Logic extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelBeta,
	)
}

func createDefaultConfig() component.Config {
	defaultCredsPath := ""

	return &Config{
		ApiBaseUrl:                    DefaultApiBaseUrl,
		HeartBeatInterval:             DefaultHeartbeatInterval,
		CollectorCredentialsDirectory: defaultCredsPath,
		Clobber:                       false,
		DiscoverCollectorTags:         true,
		ForceRegistration:             false,
		Ephemeral:                     false,
		TimeZone:                      "",
		BackOff: backOffConfig{
			InitialInterval: backoff.DefaultInitialInterval,
			MaxInterval:     backoff.DefaultMaxInterval,
			MaxElapsedTime:  backoff.DefaultMaxElapsedTime,
		},
	}
}

func createExtension(_ context.Context, params extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)
	return newSumologicExtension(config, params.Logger, params.ID, params.BuildInfo.Version)
}
