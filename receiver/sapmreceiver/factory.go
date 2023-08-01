// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sapmreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver"

// This file implements factory for SAPM receiver.

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver/internal/metadata"
)

const (
	// Default endpoints to bind to.
	defaultEndpoint = ":7276"
)

// NewFactory creates a factory for SAPM receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: defaultEndpoint,
		},
	}
}

// extract the port number from string in "address:port" format. If the
// port number cannot be extracted returns an error.
// TODO make this a utility function
func extractPortFromEndpoint(endpoint string) (int, error) {
	_, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return 0, fmt.Errorf("endpoint is not formatted correctly: %w", err)
	}
	port, err := strconv.ParseInt(portStr, 10, 0)
	if err != nil {
		return 0, fmt.Errorf("endpoint port is not a number: %w", err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port number must be between 1 and 65535")
	}
	return int(port), nil
}

// verify that the configured port is not 0
func (rCfg *Config) validate() error {
	_, err := extractPortFromEndpoint(rCfg.Endpoint)
	if err != nil {
		return err
	}
	return nil
}

// CreateTracesReceiver creates a trace receiver based on provided config.
func createTracesReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	// assert config is SAPM config
	rCfg := cfg.(*Config)

	err := rCfg.validate()
	if err != nil {
		return nil, err
	}

	// Create the receiver.
	return newReceiver(params, rCfg, nextConsumer)
}
