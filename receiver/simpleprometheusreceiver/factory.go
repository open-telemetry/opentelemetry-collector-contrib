// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package simpleprometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/internal/metadata"
)

// This file implements factory for prometheus_simple receiver
const (
	defaultEndpoint    = "localhost:9090"
	defaultMetricsPath = "/metrics"
)

var defaultCollectionInterval = 10 * time.Second

// NewFactory creates a factory for "Simple" Prometheus receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: defaultEndpoint,
			TLSSetting: configtls.TLSClientSetting{
				Insecure: true,
			},
		},
		MetricsPath:        defaultMetricsPath,
		CollectionInterval: defaultCollectionInterval,
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	rCfg := cfg.(*Config)
	return new(params, rCfg, nextConsumer), nil
}
