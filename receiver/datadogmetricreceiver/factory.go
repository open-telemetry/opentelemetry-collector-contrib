// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogmetricreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/internal/metadata"
)

// NewFactory creates a factory for DataDog receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricStability))

}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "localhost:8122",
		},
		ReadTimeout: 60 * time.Second,
	}
}

func createMetricsReceiver(_ context.Context, params receiver.CreateSettings, cfg component.Config, consumer consumer.Metrics) (r receiver.Metrics, err error) {
	rcfg := cfg.(*Config)
	r = receivers.GetOrAdd(cfg, func() component.Component {
		dd, _ := newdatadogmetricreceiver(rcfg, consumer, params)
		return dd
	})
	return r, nil
}

var receivers = sharedcomponent.NewSharedComponents()
