// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mockawsxrayreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatareceivers/mockawsxrayreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

// This file implements factory for awsxray receiver.

const (
	// The value of "type" key in configuration.
	typeStr = "mock_receiver"
	// stability level of test component
	stability = component.StabilityLevelDevelopment

	// Default endpoints to bind to.
	defaultEndpoint = ":7276"
)

// NewFactory creates a factory for SAPM receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, stability))
}

// CreateDefaultConfig creates the default configuration for Jaeger receiver.
func createDefaultConfig() component.Config {
	return &Config{
		Endpoint: defaultEndpoint,
	}
}

// CreateTracesReceiver creates a trace receiver based on provided config.
func createTracesReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	rCfg := cfg.(*Config)
	return New(nextConsumer, params, rCfg)
}
