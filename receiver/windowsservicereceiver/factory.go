// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//revive:disable:unused-parameter

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver/internal/metadata"
)

func createDefaultConfig() component.Config {
	return &Config{}
}

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createMetricsReceiver(
	_ context.Context,
	_ receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	rcvr := newMetricsReceiver(cfg, consumer)
	return rcvr, nil
}

func newMetricsReceiver(*Config, consumer.Metrics) *windowsServiceReceiver {
	return &windowsServiceReceiver{}
}

type windowsServiceReceiver struct{}

func (*windowsServiceReceiver) Start(context.Context, component.Host) error {
	return nil
}

func (*windowsServiceReceiver) Shutdown(context.Context) error {
	return nil
}
