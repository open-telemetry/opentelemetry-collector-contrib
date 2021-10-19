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

package humioexporter

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The key used to refer to this exporter
	typeStr = "humio"
)

// NewFactory creates an exporter factory for Humio
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTracesExporter),
	)
}

// Provides a struct with default values for all relevant configuration settings
func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),

		// Default settings inherited from exporter helper
		QueueSettings: exporterhelper.DefaultQueueSettings(),
		RetrySettings: exporterhelper.DefaultRetrySettings(),

		HTTPClientSettings: confighttp.HTTPClientSettings{
			Headers: map[string]string{},
		},

		// Settings specific to the Humio exporter
		DisableCompression: false,
		Tag:                TagNone,
		Traces: TracesConfig{
			UnixTimestamps: false,
		},
	}
}

// Creates a new trace exporter for Humio
func createTracesExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (component.TracesExporter, error) {
	if config == nil {
		return nil, errors.New("missing config")
	}
	cfg := config.(*Config)

	if err := cfg.sanitize(); err != nil {
		return nil, err
	}

	// We only require the trace ingest token when the trace exporter is enabled
	if cfg.Traces.IngestToken == "" {
		return nil, errors.New("an ingest token for traces is required when enabling the Humio trace exporter")
	}

	exporter := newTracesExporter(cfg, set.Logger)

	return exporterhelper.NewTracesExporter(
		cfg,
		set,
		exporter.pushTraceData,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithStart(exporter.start),
		exporterhelper.WithShutdown(exporter.shutdown),
	)
}
