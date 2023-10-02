// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr        = "sumologic"
	stabilityLevel = component.StabilityLevelBeta
)

// NewFactory returns a new factory for the sumologic exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, stabilityLevel),
		exporter.WithMetrics(createMetricsExporter, stabilityLevel),
		exporter.WithTraces(createTracesExporter, stabilityLevel),
	)
}

func createDefaultConfig() component.Config {
	qs := exporterhelper.NewDefaultQueueSettings()
	qs.Enabled = false

	return &Config{
		CompressEncoding:   DefaultCompressEncoding,
		MaxRequestBodySize: DefaultMaxRequestBodySize,
		LogFormat:          DefaultLogFormat,
		MetricFormat:       DefaultMetricFormat,
		Client:             DefaultClient,
		ClearLogsTimestamp: DefaultClearLogsTimestamp,
		JSONLogs: JSONLogs{
			LogKey:       DefaultLogKey,
			AddTimestamp: DefaultAddTimestamp,
			TimestampKey: DefaultTimestampKey,
			FlattenBody:  DefaultFlattenBody,
		},
		TraceFormat: OTLPTraceFormat,

		HTTPClientSettings:   createDefaultHTTPClientSettings(),
		RetrySettings:        exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:        qs,
		DropRoutingAttribute: DefaultDropRoutingAttribute,
	}
}

func createLogsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	exp, err := newLogsExporter(ctx, params, cfg.(*Config))
	if err != nil {
		return nil, fmt.Errorf("failed to create the logs exporter: %w", err)
	}

	return exp, nil
}

func createMetricsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	exp, err := newMetricsExporter(ctx, params, cfg.(*Config))
	if err != nil {
		return nil, fmt.Errorf("failed to create the metrics exporter: %w", err)
	}

	return exp, nil
}

func createTracesExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg component.Config,
) (exporter.Traces, error) {
	exp, err := newTracesExporter(ctx, params, cfg.(*Config))
	if err != nil {
		return nil, fmt.Errorf("failed to create the traces exporter: %w", err)
	}

	return exp, nil
}
