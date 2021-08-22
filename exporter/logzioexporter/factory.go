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

package logzioexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const typeStr = "logzio"

// NewFactory creates a factory for Logz.io exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTracesExporter))
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
		Region:           "",
		TracesToken:      "",
		MetricsToken:     "",
		DrainInterval:    3,
		QueueMaxLength:   500000,
		QueueCapacity:    20 * 1024 * 1024,
	}
}

func createTracesExporter(_ context.Context, params component.ExporterCreateSettings, cfg config.Exporter) (component.TracesExporter, error) {
	config := cfg.(*Config)
	return newLogzioTracesExporter(config, params)
}

func createMetricsExporter(_ context.Context, params component.ExporterCreateSettings, cfg config.Exporter) (component.MetricsExporter, error) {
	config := cfg.(*Config)
	return newLogzioMetricsExporter(config, params)
}
