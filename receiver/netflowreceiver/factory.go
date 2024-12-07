// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver/internal/metadata"
)

const (
	defaultSockets   = 1
	defaultWorkers   = 2
	defaultQueueSize = 1_000_000
)

// NewFactory creates a factory for netflow receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		Scheme:    "netflow",
		Port:      2055,
		Sockets:   defaultSockets,
		Workers:   defaultWorkers,
		QueueSize: defaultQueueSize,
	}
}

func createLogsReceiver(_ context.Context, params receiver.Settings, cfg component.Config, consumer consumer.Logs) (receiver.Logs, error) {
	logger := params.Logger
	conf := cfg.(*Config)

	nr := &netflowReceiver{
		logger:      logger,
		logConsumer: consumer,
		config:      conf,
	}

	return nr, nil
}
