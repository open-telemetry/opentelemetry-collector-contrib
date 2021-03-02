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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

const typeStr = "newrelic"

// NewFactory creates a factory for New Relic exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTraceExporter),
		exporterhelper.WithMetrics(createMetricsExporter))
}

func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		Timeout: time.Second * 15,
	}
}

// CreateTracesExporter creates a New Relic trace exporter for this configuration.
func createTraceExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.TracesExporter, error) {
	exp, err := newTraceExporter(params.Logger, cfg)
	if err != nil {
		return nil, err
	}

	// The logger is only used in a disabled queuedRetrySender, which noisily logs at
	// the error level when it is disabled and errors occur.
	return exporterhelper.NewTraceExporter(cfg, zap.NewNop(), exp.pushTraceData,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: cfg.(*Config).Timeout}),
		exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}))
}

// CreateMetricsExporter creates a New Relic metrics exporter for this configuration.
func createMetricsExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.MetricsExporter, error) {
	exp, err := newMetricsExporter(params.Logger, cfg)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(cfg, params.Logger, exp.pushMetricData, exporterhelper.WithShutdown(exp.Shutdown))
}
