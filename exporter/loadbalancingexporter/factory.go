// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
)

const (
	// The value of "type" key in configuration.
	typeStr = "loadbalancing"
)

// NewFactory creates a factory for the exporter.
func NewFactory() component.ExporterFactory {
	view.Register(MetricViews()...)

	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTracesExporter),
		exporterhelper.WithLogs(createLogExporter),
	)
}

func createDefaultConfig() config.Exporter {
	otlpFactory := otlpexporter.NewFactory()
	otlpDefaultCfg := otlpFactory.CreateDefaultConfig().(*otlpexporter.Config)

	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		Protocol: Protocol{
			OTLP: *otlpDefaultCfg,
		},
	}
}

func createTracesExporter(_ context.Context, params component.ExporterCreateSettings, cfg config.Exporter) (component.TracesExporter, error) {
	return newTracesExporter(params, cfg)
}

func createLogExporter(_ context.Context, params component.ExporterCreateSettings, cfg config.Exporter) (component.LogsExporter, error) {
	return newLogsExporter(params, cfg)
}
