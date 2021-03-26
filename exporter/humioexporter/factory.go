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

package humioexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The key used to refer to this exporter
	typeStr = "humio"
)

// Creates an exporter factory for Humio
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTraceExporter),
		// To be added over time
		// exporterhelper.WithMetrics(createMetricsExporter),
		// exporterhelper.WithLogs(createLogsExporter),
	)
}

// Provides a struct with default values for all relevant configuration settings
func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},

		// Default settings inherited from exporter helper
		QueueSettings: exporterhelper.DefaultQueueSettings(),
		RetrySettings: exporterhelper.DefaultRetrySettings(),

		// Settings specific to the Humio exporter
		Endpoint:         "localhost:8080",
		Tags:             map[string]string{},
		EnableServiceTag: true,
		Traces: TracesConfig{
			IsoTimestamps:    true,
			EnableRawstrings: false,
		},
	}
}

// Creates a new trace exporter for Humio
func createTraceExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter,
) (component.TracesExporter, error) {
	return nil, nil
}

// Creates a new metrics exporter for Humio
func createMetricsExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter,
) (component.MetricsExporter, error) {
	return nil, nil
}

// Creates a new logs exporter for Humio
func createLogsExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter,
) (component.LogsExporter, error) {
	return nil, nil
}
