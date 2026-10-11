// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xreceiver.WithProfiles(createProfilesReceiver, metadata.ProfilesStability),
	)
}

func defaultControllerConfig() scraperhelper.ControllerConfig {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 10 * time.Second
	return cfg
}

func createDefaultConfig() component.Config {
	return &Config{
		Remote: configoptional.Default(RemoteConfig{
			ControllerConfig: defaultControllerConfig(),
			ClientConfig:     confighttp.NewDefaultClientConfig(),
		}),
		File: configoptional.Default(FileConfig{
			ControllerConfig: defaultControllerConfig(),
		}),
		Self: configoptional.Default(SelfConfig{
			ControllerConfig:     defaultControllerConfig(),
			BlockProfileFraction: 1,
			MutexProfileFraction: 1,
		}),
		Server: configoptional.Default(ServerConfig{
			ServerConfig: confighttp.NewDefaultServerConfig(),
		}),
	}
}

func createProfilesReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer xconsumer.Profiles,
) (xreceiver.Profiles, error) {
	rCfg := cfg.(*Config)
	return newReceiver(rCfg, settings, consumer)
}
