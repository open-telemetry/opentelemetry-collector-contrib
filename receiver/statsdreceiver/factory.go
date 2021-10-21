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

package statsdreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

const (
	// The value of "type" key in configuration.
	typeStr                    = "statsd"
	defaultBindEndpoint        = "localhost:8125"
	defaultTransport           = "udp"
	defaultAggregationInterval = 60 * time.Second
	defaultEnableMetricType    = false
	defaultIsMonotonicCounter  = false
)

var (
	defaultTimerHistogramMapping = []protocol.TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}}
)

// NewFactory creates a factory for the StatsD receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver),
	)
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		NetAddr: confignet.NetAddr{
			Endpoint:  defaultBindEndpoint,
			Transport: defaultTransport,
		},
		AggregationInterval:   defaultAggregationInterval,
		EnableMetricType:      defaultEnableMetricType,
		IsMonotonicCounter:    defaultIsMonotonicCounter,
		TimerHistogramMapping: defaultTimerHistogramMapping,
	}
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	c := cfg.(*Config)
	err := c.validate()
	if err != nil {
		return nil, err
	}
	return New(params, *c, consumer)
}
