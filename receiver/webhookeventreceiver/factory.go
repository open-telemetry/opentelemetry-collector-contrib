// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver/internal/metadata"
)

const (
	// The value of "type" key in configuration.
	typeStr = "webhookevent"
	// The stability level of the receiver.
	stability = component.StabilityLevelDevelopment
    // might add this later, for now I wish to require a valid 
    // endpoint to be declared by the user.
	// Default endpoints to bind to.
    // defaultEndpoint = "localhost:8080"
    defaultReadTimeout = "500"
    defaultWriteTimeout = "500"
    defaultPath = "/events"
    defaultHealthPath = "/health_check"
)

// NewFactory creates a factory for Generic Webhook Receiver.
func NewFactory() component.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

// Default configuration for the generic webhook receiver
func createDefaultConfig() component.Config {
	return &Config{
        Path: defaultPath,
        HealthPath:   defaultHealthPath,
        ReadTimeout:  defaultReadTimeout,
        WriteTimeout: defaultWriteTimeout,
	}
}

// createLogsReceiver creates a logs receiver based on provided config.
func createLogsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	conf := cfg.(*Config)
    rec, err := newLogsReceiver(params, *conf, consumer)
    if err != nil {
        return nil, err
    }

	return rec, nil
}
