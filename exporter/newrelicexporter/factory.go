// Copyright The OpenTelemetry Authors
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

package newrelicexporter

import (
	"context"
	"fmt"

	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

const typeStr = "newrelic"

// NewFactory creates a factory for New Relic exporter.
func NewFactory() component.ExporterFactory {
	view.Register(MetricViews()...)

	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTracesExporter),
		exporterhelper.WithMetrics(createMetricsExporter),
		exporterhelper.WithLogs(createLogsExporter),
	)
}

func createDefaultConfig() config.Exporter {
	defaultRetry := exporterhelper.DefaultRetrySettings()
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),

		CommonConfig: EndpointConfig{
			TimeoutSettings: exporterhelper.DefaultTimeoutSettings(),
			RetrySettings:   defaultRetry,
		},
	}
}

// CreateTracesExporter creates a New Relic trace exporter for this configuration.
func createTracesExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.TracesExporter, error) {
	nrConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", cfg)
	}
	traceConfig := nrConfig.TracesConfig
	exp, err := newExporter(set.Logger, &set.BuildInfo, traceConfig, telemetry.NewSpanRequestFactory)
	if err != nil {
		return nil, err
	}

	// The logger is only used in a disabled queuedRetrySender, which noisily logs at
	// the error level when it is disabled and errors occur.
	set.Logger = zap.NewNop()
	return exporterhelper.NewTracesExporter(cfg, set, exp.pushTraceData,
		exporterhelper.WithTimeout(traceConfig.TimeoutSettings),
		exporterhelper.WithRetry(traceConfig.RetrySettings),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
	)
}

// CreateMetricsExporter creates a New Relic metrics exporter for this configuration.
func createMetricsExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.MetricsExporter, error) {
	nrConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", cfg)
	}

	metricsConfig := nrConfig.MetricsConfig
	exp, err := newExporter(set.Logger, &set.BuildInfo, metricsConfig, telemetry.NewMetricRequestFactory)
	if err != nil {
		return nil, err
	}

	// The logger is only used in a disabled queuedRetrySender, which noisily logs at
	// the error level when it is disabled and errors occur.
	set.Logger = zap.NewNop()
	return exporterhelper.NewMetricsExporter(cfg, set, exp.pushMetricData,
		exporterhelper.WithTimeout(metricsConfig.TimeoutSettings),
		exporterhelper.WithRetry(metricsConfig.RetrySettings),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
	)
}

// CreateLogsExporter creates a New Relic logs exporter for this configuration.
func createLogsExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.LogsExporter, error) {
	nrConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", cfg)
	}

	logsConfig := nrConfig.LogsConfig
	exp, err := newExporter(set.Logger, &set.BuildInfo, logsConfig, telemetry.NewLogRequestFactory)
	if err != nil {
		return nil, err
	}

	// The logger is only used in a disabled queuedRetrySender, which noisily logs at
	// the error level when it is disabled and errors occur.
	set.Logger = zap.NewNop()
	return exporterhelper.NewLogsExporter(cfg, set, exp.pushLogData,
		exporterhelper.WithTimeout(logsConfig.TimeoutSettings),
		exporterhelper.WithRetry(logsConfig.RetrySettings),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
	)
}
