// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

//go:build aix

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"
import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter/internal/metadata"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		func() component.Config {
			return nil
		},
		exporter.WithMetrics(createMetrics, metadata.MetricsStability),
		exporter.WithTraces(createTraces, metadata.TracesStability),
		exporter.WithLogs(createLogs, metadata.LogsStability),
	)
}

func createMetrics(context.Context, exporter.Settings, component.Config) (exporter.Metrics, error) {
	return nil, errors.New("pulsarexporter is not supported on aix")
}

func createTraces(context.Context, exporter.Settings, component.Config) (exporter.Traces, error) {
	return nil, errors.New("pulsarexporter is not supported on aix")
}

func createLogs(context.Context, exporter.Settings, component.Config) (exporter.Logs, error) {
	return nil, errors.New("pulsarexporter is not supported on aix")
}
