// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter/internal/metadata"
)

const (
	defaultEncoding = "otlp_proto"
	defaultBroker   = "pulsar://localhost:6650"
)

// NewFactory creates Pulsar exporter factory.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings:         exporterhelper.NewDefaultTimeoutSettings(),
		RetrySettings:           exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:           exporterhelper.NewDefaultQueueSettings(),
		Trace:                   ExporterOption{Encoding: defaultEncoding},
		Log:                     ExporterOption{Encoding: defaultEncoding},
		Metric:                  ExporterOption{Encoding: defaultEncoding},
		Endpoint:                defaultBroker,
		Authentication:          Authentication{},
		MaxConnectionsPerBroker: 1,
		ConnectionTimeout:       5 * time.Second,
		OperationTimeout:        30 * time.Second,
	}
}
