// Copyright 2019, OpenTelemetry Authors
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

package cloudfoundryreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// This file implements factory for Cloud Foundry receiver.

const (
	typeStr                  = "cloudfoundry"
	defaultUAAUsername       = "admin"
	defaultRLPGatewayShardID = "opentelemetry"
	defaultURL               = "https://localhost"
)

// NewFactory creates a factory for collectd receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings:        config.NewReceiverSettings(config.NewComponentID(typeStr)),
		RLPGatewayURL:           defaultURL,
		RLPGatewaySkipTLSVerify: false,
		RLPGatewayShardID:       defaultRLPGatewayShardID,
		UAAUsername:             defaultUAAUsername,
		UAAPassword:             "",
		UAAUrl:                  defaultURL,
		UAASkipTLSVerify:        false,
	}
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	c := cfg.(*Config)
	return newCloudFoundryReceiver(params.Logger, *c, nextConsumer)
}
