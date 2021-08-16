// Copyright 2020, OpenTelemetry Authors
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

package simpleprometheusreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// This file implements factory for prometheus_simple receiver
const (
	// The value of "type" key in configuration.
	typeStr = "prometheus_simple"

	defaultEndpoint    = "localhost:9090"
	defaultMetricsPath = "/metrics"
)

var defaultCollectionInterval = 10 * time.Second

// NewFactory creates a factory for "Simple" Prometheus receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewID(typeStr)),
		TCPAddr: confignet.TCPAddr{
			Endpoint: defaultEndpoint,
		},
		MetricsPath:        defaultMetricsPath,
		CollectionInterval: defaultCollectionInterval,
	}
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	rCfg := cfg.(*Config)
	return new(params, rCfg, nextConsumer), nil
}
