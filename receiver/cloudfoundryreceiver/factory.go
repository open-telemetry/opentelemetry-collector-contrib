// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

// This file implements factory for Cloud Foundry receiver.

const (
	typeStr                  = "cloudfoundry"
	stability                = component.StabilityLevelBeta
	defaultUAAUsername       = "admin"
	defaultRLPGatewayShardID = "opentelemetry"
	defaultURL               = "https://localhost"
)

// NewFactory creates a factory for collectd receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
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
