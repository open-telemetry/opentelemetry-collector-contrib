// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver"

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

const (
	defaultBindEndpoint        = "localhost:8125"
	defaultAggregationInterval = 60 * time.Second
	defaultEnableMetricType    = false
	defaultIsMonotonicCounter  = false
	defaultSocketPermissions   = os.FileMode(0o622)
)

var defaultTimerHistogramMapping = []protocol.TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}, {StatsdType: "distribution", ObserverType: "gauge"}}

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
		NetAddr: confignet.AddrConfig{
			Endpoint:  defaultBindEndpoint,
			Transport: confignet.TransportTypeUDP,
		},
		AggregationInterval:   defaultAggregationInterval,
		EnableMetricType:      defaultEnableMetricType,
		IsMonotonicCounter:    defaultIsMonotonicCounter,
		TimerHistogramMapping: defaultTimerHistogramMapping,
		SocketPermissions:     defaultSocketPermissions,
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	return newReceiver(params, *c, consumer)
}
