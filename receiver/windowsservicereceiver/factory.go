// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowsservicereceiver // import "github.com/open-telemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"context"

	"github.com/open-telemetry-collector-contrib/receiver/windowsservicereceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func createDefaultConfig() component.Config {
	return &Config{}
}

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
	)
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	rcvr := newMetricsReceiver(cfg, consumer)
	return rcvr, nil
}

func newMetricsReceiver(cfg *Config, consumer consumer.Metrics) *windowsServiceReceiver {
	return &windowsServiceReceiver{}
}

type windowsServiceReceiver struct{}

func (r *windowsServiceReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (r *windowsServiceReceiver) Shutdown(ctx context.Context) error {
	return nil
}
