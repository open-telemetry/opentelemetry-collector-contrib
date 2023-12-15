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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/protocol"
)

const (
	defaultBindEndpoint        = "localhost:8125"
	defaultTransport           = "udp"
	defaultAggregationInterval = 60 * time.Second
	defaultEnableMetricType    = false
	defaultIsMonotonicCounter  = false
)

var (
	defaultTimerHistogramMapping = []protocol.TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}, {StatsdType: "distribution", ObserverType: "gauge"}}
)

// NewFactory creates a factory for the StatsD receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
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
	return newReceiver(params, *c, consumer)
}
