// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver/internal/metadata"
)

// This file implements factory for CollectD receiver.

const (
	defaultBindEndpoint   = "localhost:8081"
	defaultEncodingFormat = "json"
)

// NewFactory creates a factory for collectd receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}
func createDefaultConfig() component.Config {
	return &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: defaultBindEndpoint,
		},
		Timeout:  30 * time.Second,
		Encoding: defaultEncodingFormat,
	}
}

func createMetricsReceiver(
	_ context.Context,
	cs receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	return newCollectdReceiver(cs.Logger, c, c.AttributesPrefix, nextConsumer, cs)
}
