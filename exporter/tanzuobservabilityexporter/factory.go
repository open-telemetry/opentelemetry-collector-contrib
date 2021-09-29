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

package tanzuobservabilityexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const exporterType = "tanzuobservability"

// NewFactory creates a factory for the exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		exporterType,
		createDefaultConfig,
		exporterhelper.WithTraces(createTracesExporter),
	)
}

func createDefaultConfig() config.Exporter {
	tracesCfg := TracesConfig{
		HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost:30001"},
	}
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(exporterType)),
		QueueSettings:    exporterhelper.DefaultQueueSettings(),
		RetrySettings:    exporterhelper.DefaultRetrySettings(),
		Traces:           tracesCfg,
	}
}

// createTracesExporter implements exporterhelper.CreateTracesExporter and creates
// an exporter for traces using this configuration
func createTracesExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.TracesExporter, error) {
	exp, err := newTracesExporter(set.Logger, cfg)
	if err != nil {
		return nil, err
	}

	tobsCfg := cfg.(*Config)

	return exporterhelper.NewTracesExporter(
		cfg,
		set,
		exp.pushTraceData,
		exporterhelper.WithQueue(tobsCfg.QueueSettings),
		exporterhelper.WithRetry(tobsCfg.RetrySettings),
		exporterhelper.WithShutdown(exp.shutdown),
	)
}
