// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver/internal/metadata"
)

// This file implements factory for Cloud Foundry receiver.

const (
	defaultUAAUsername       = "admin"
	defaultRLPGatewayShardID = "opentelemetry"
	defaultURL               = "https://localhost"
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
		RLPGateway: RLPGatewayConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: defaultURL,
				TLSSetting: configtls.TLSClientSetting{
					InsecureSkipVerify: false,
				},
			},
			ShardID: defaultRLPGatewayShardID,
		},
		UAA: UAAConfig{
			LimitedHTTPClientSettings: LimitedHTTPClientSettings{
				Endpoint: defaultURL,
				TLSSetting: LimitedTLSClientSetting{
					InsecureSkipVerify: false,
				},
			},
			Username: defaultUAAUsername,
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	return newCloudFoundryReceiver(params, *c, nextConsumer)
}
