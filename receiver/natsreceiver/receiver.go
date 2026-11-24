// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/natsreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

// natsReceiver consumes telemetry from a NATS server. One instance is shared by
// the logs, metrics, and traces pipelines of a configuration, so all signals
// use a single NATS connection.
//
// NOTE: this is a skeleton. Connection management and the subscribe paths are
// intentionally unimplemented and land in follow-up PRs (see the component
// donation issue). Start and Shutdown are currently no-ops.
type natsReceiver struct {
	config   *Config
	settings receiver.Settings

	// Downstream consumers; nil for signals not used in any pipeline.
	nextLogs    consumer.Logs
	nextMetrics consumer.Metrics
	nextTraces  consumer.Traces
}

func newReceiver(set receiver.Settings, cfg *Config) *natsReceiver {
	return &natsReceiver{
		config:   cfg,
		settings: set,
	}
}

func (*natsReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*natsReceiver) Shutdown(_ context.Context) error {
	return nil
}
