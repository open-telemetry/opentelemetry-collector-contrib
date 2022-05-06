// Copyright  The OpenTelemetry Authors
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

package mezmoexporter

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const typeStr = "mezmo"

// NewFactory creates a factory for Mezmo exporter.
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithLogsExporter(createLogsExporter),
	)
}

// Create a default Memzo config
func createDefaultConfig() config.Exporter {
	qs := exporterhelper.NewDefaultQueueSettings()
	qs.Enabled = false

	return &Config{
		ExporterSettings:   config.NewExporterSettings(config.NewComponentID(typeStr)),
		HTTPClientSettings: createDefaultHTTPClientSettings(),
		RetrySettings:      exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:      qs,
	}
}

// Create a log exporter for exporting to Mezmo
func createLogsExporter(ctx context.Context, settings component.ExporterCreateSettings, exporter config.Exporter) (component.LogsExporter, error) {
	if exporter == nil {
		return nil, errors.New("nil config")
	}
	expCfg := exporter.(*Config)

	if err := expCfg.Validate(); err != nil {
		return nil, err
	}

	exp := newLogsExporter(expCfg, settings.TelemetrySettings)

	return exporterhelper.NewLogsExporter(
		expCfg,
		settings,
		exp.pushLogData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(expCfg.RetrySettings),
		exporterhelper.WithQueue(expCfg.QueueSettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.stop),
	)
}
