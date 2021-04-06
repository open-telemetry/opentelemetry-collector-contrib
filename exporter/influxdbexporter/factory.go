// Copyright 2021, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package influxdbexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// NewFactory creates a factory for Jaeger Thrift over HTTP exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTraceExporter),
		exporterhelper.WithMetrics(createMetricsExporter),
		exporterhelper.WithLogs(createLogsExporter),
	)
}

func createTraceExporter(ctx context.Context, params component.ExporterCreateParams, config config.Exporter) (component.TracesExporter, error) {
	cfg := config.(*Config)
	println(cfg)

	exporter, err := newExporter(cfg, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTraceExporter(
		config,
		params.Logger,
		exporter.pushTraces,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithResourceToTelemetryConversion(exporterhelper.ResourceToTelemetrySettings{Enabled: true}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)
}

func createMetricsExporter(ctx context.Context, params component.ExporterCreateParams, config config.Exporter) (component.MetricsExporter, error) {
	cfg := config.(*Config)

	exporter, err := newExporter(cfg, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(
		config,
		params.Logger,
		exporter.pushMetrics,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithResourceToTelemetryConversion(exporterhelper.ResourceToTelemetrySettings{Enabled: true}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)
}

func createLogsExporter(ctx context.Context, params component.ExporterCreateParams, config config.Exporter) (component.LogsExporter, error) {
	cfg := config.(*Config)

	exporter, err := newExporter(cfg, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogsExporter(
		config,
		params.Logger,
		exporter.pushLogs,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithResourceToTelemetryConversion(exporterhelper.ResourceToTelemetrySettings{Enabled: true}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.ExporterSettings{
			TypeVal: typeStr,
		},
		QueueSettings:   exporterhelper.DefaultQueueSettings(),
		RetrySettings:   exporterhelper.DefaultRetrySettings(),
		TimeoutSettings: exporterhelper.DefaultTimeoutSettings(),

		Protocol: protocolLineProtocol,
	}
}
