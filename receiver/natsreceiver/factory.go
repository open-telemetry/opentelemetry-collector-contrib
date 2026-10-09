// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/natsreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/natsclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/natsreceiver/internal/metadata"
)

// Default subjects match the NATS exporter's defaults.
const (
	defaultLogsSubject    = "otel_logs"
	defaultMetricsSubject = "otel_metrics"
	defaultTracesSubject  = "otel_traces"
)

// NewFactory creates a factory for the NATS receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ClientConfig: natsclient.NewDefaultClientConfig(),
		Logs:         SignalConfig{Subject: defaultLogsSubject},
		Metrics:      SignalConfig{Subject: defaultMetricsSubject},
		Traces:       SignalConfig{Subject: defaultTracesSubject},
	}
}

// receivers caches one natsReceiver per configuration so the logs, metrics,
// and traces pipelines share a single instance (and a single NATS connection)
// instead of creating one per signal.
var receivers = sharedcomponent.NewSharedComponents()

func getOrCreateReceiver(set receiver.Settings, cfg component.Config) *sharedcomponent.SharedComponent {
	return receivers.GetOrAdd(cfg, func() component.Component {
		return newReceiver(set, cfg.(*Config))
	})
}

func createLogsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	next consumer.Logs,
) (receiver.Logs, error) {
	sc := getOrCreateReceiver(set, cfg)
	sc.Unwrap().(*natsReceiver).nextLogs = next
	return sc, nil
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (receiver.Metrics, error) {
	sc := getOrCreateReceiver(set, cfg)
	sc.Unwrap().(*natsReceiver).nextMetrics = next
	return sc, nil
}

func createTracesReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	next consumer.Traces,
) (receiver.Traces, error) {
	sc := getOrCreateReceiver(set, cfg)
	sc.Unwrap().(*natsReceiver).nextTraces = next
	return sc, nil
}
