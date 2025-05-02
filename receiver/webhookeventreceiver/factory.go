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

var scopeLogName = "otlp/" + metadata.Type.String()

const (
	// might add this later, for now I wish to require a valid
	// endpoint to be declared by the user.
	// Default endpoints to bind to.
	// defaultEndpoint = "localhost:8080"
	defaultReadTimeout  = "500ms"
	defaultWriteTimeout = "500ms"
	defaultPath         = "/events"
	defaultHealthPath   = "/health_check"
)

// NewFactory creates a factory for Generic Webhook Receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

// Default configuration for the generic webhook receiver
func createDefaultConfig() component.Config {
	return &Config{
		Path:                       defaultPath,
		HealthPath:                 defaultHealthPath,
		ReadTimeout:                defaultReadTimeout,
		WriteTimeout:               defaultWriteTimeout,
		ConvertHeadersToAttributes: false, // optional, off by default
		SplitLogsAtNewLine:         false,
	}
}

// createLogsReceiver creates a logs receiver based on provided config.
func createLogsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	conf := cfg.(*Config)
	return newLogsReceiver(params, *conf, consumer)
}
