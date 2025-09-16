// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	stability                 = component.StabilityLevelBeta
	defaultCollectionInterval = 30 * time.Second
	defaultTimeout            = 30 * time.Second
)

var typeStr = component.MustNewType("ciscoosreceiver")

// NewFactory creates a factory for Cisco receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		CollectionInterval: 60 * time.Second,
		Devices:            []DeviceConfig{},
		Timeout:            30 * time.Second,
		Collectors: CollectorsConfig{
			BGP:         true,
			Environment: true,
			Facts:       true,
			Interfaces:  true,
			Optics:      true,
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	conf := cfg.(*Config)
	return newModularCiscoReceiver(conf, set, consumer)
}
