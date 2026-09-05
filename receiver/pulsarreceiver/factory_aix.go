// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build aix

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"
import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver/internal/metadata"
)

// NewFactory creates Pulsar exporter factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(metadata.Type, func() component.Config {
		return nil
	}, receiver.WithLogs(createLogs, metadata.LogsStability),
		receiver.WithMetrics(createMetrics, metadata.MetricsStability),
		receiver.WithTraces(createTraces, metadata.TracesStability))
}

func createTraces(context.Context, receiver.Settings, component.Config, consumer.Traces) (receiver.Traces, error) {
	return nil, errors.New("pulsarreceiver is not supported on AIX")
}

func createMetrics(context.Context, receiver.Settings, component.Config, consumer.Metrics) (receiver.Metrics, error) {
	return nil, errors.New("pulsarreceiver is not supported on AIX")
}

func createLogs(context.Context, receiver.Settings, component.Config, consumer.Logs) (receiver.Logs, error) {
	return nil, errors.New("pulsarreceiver is not supported on AIX")
}
