// Copyright 2019 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package honeycombexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "honeycomb"
)

// NewFactory creates a factory for Honeycomb exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTracesExporter))
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings:    config.NewExporterSettings(config.NewComponentID(typeStr)),
		APIKey:              "",
		Dataset:             "",
		APIURL:              "https://api.honeycomb.io",
		SampleRateAttribute: "",
		Debug:               false,
		RetrySettings:       exporterhelper.DefaultRetrySettings(),
		QueueSettings:       exporterhelper.DefaultQueueSettings(),
	}
}

func createTracesExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.TracesExporter, error) {
	eCfg := cfg.(*Config)
	exporter, err := newHoneycombTracesExporter(eCfg, set.Logger)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(
		cfg,
		set,
		exporter.pushTraceData,
		exporterhelper.WithShutdown(exporter.Shutdown),
		exporterhelper.WithRetry(eCfg.RetrySettings),
		exporterhelper.WithQueue(eCfg.QueueSettings))
}
