// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"context"
	"errors"

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
		ClientConfig: confighttp.NewDefaultClientConfig(),
	}
}

func createProfilesReceiver(
	_ context.Context,
	_ receiver.Settings,
	_ component.Config,
	_ xconsumer.Profiles,
) (xreceiver.Profiles, error) {
	return &rcvr{}, errors.New("not implemented")
}

type rcvr struct{}

func (p rcvr) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (p rcvr) Shutdown(_ context.Context) error {
	return nil
}
