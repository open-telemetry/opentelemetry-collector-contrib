// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

const (
	// The value of "type" key in configuration.
	typeStr                    = "statsd"
	stability                  = component.StabilityLevelBeta
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
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
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
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	return New(params, *c, consumer)
}
