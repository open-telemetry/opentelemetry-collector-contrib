// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xreceiver.WithProfiles(createProfilesReceiver, metadata.ProfilesStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ClientConfig:       confighttp.NewDefaultClientConfig(),
		CollectionInterval: 10 * time.Second,
	}
}

func createProfilesReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	c xconsumer.Profiles,
) (xreceiver.Profiles, error) {
	return &pprofReceiver{
		consumer:          c,
		telemetrySettings: set.TelemetrySettings,
		config:            cfg.(*Config),
		done:              make(chan struct{}),
	}, nil
}
