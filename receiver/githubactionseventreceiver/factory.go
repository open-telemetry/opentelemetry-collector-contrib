// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionseventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubactionseventreceiver/internal/metadata"
)

// This file implements factory for GitHub Actions Event receiver.

const (
	defaultBindEndpoint = "0.0.0.0:19418"
	defaultPath         = "/events"
)

// NewFactory creates a new GitHub Actions Event receiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability),
	)
}

// createDefaultConfig creates the default configuration for GitHub Actions Event receiver.
func createDefaultConfig() component.Config {
	return &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: defaultBindEndpoint,
		},
		Path:   defaultPath,
		Secret: "",
	}
}

// createTracesReceiver creates a trace receiver based on provided config.
func createTracesReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	rCfg := cfg.(*Config)
	return newReceiver(rCfg, set, nextConsumer)
}
