// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnmireceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	// typeStr is the unique identifier for our receiver in the collector config.
	typeStr = "gnmi"

	// Default values for the receiver.
	defaultRedial   = 10 * time.Second
	defaultTimeout  = 30 * time.Second
	defaultEncoding = "proto"
)

// NewFactory creates a factory for the gNMI receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelAlpha),
	)
}

// createDefaultConfig defines the initial state of the config before user overrides.
func createDefaultConfig() component.Config {
	return &Config{
		// We return a pointer to our Config struct defined in config.go.
		Targets: []TargetConfig{},
	}
}

// createMetricsReceiver instantiates the actual receiver object.
// It connects the configuration to the next step in the pipeline (consumer).
func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	// Cast the generic config to our specific gnmireceiver.Config.
	c := cfg.(*Config)

	// We will define the 'gnmiReceiver' struct in the next step (receiver.go).
	return newGNMIReceiver(c, set, nextConsumer)
}
