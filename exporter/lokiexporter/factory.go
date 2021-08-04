// Copyright The OpenTelemetry Authors
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

package lokiexporter

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const typeStr = "loki"

// NewFactory creates a factory for Loki exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithLogs(createLogsExporter),
	)
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "",
			Timeout:  30 * time.Second,
			Headers:  map[string]string{},
			// We almost read 0 bytes, so no need to tune ReadBufferSize.
			WriteBufferSize: 512 * 1024,
		},
		RetrySettings: exporterhelper.DefaultRetrySettings(),
		QueueSettings: exporterhelper.DefaultQueueSettings(),
		TenantID:      "",
		Labels: LabelsConfig{
			Attributes:         map[string]string{},
			ResourceAttributes: map[string]string{},
		},
	}
}

func createLogsExporter(_ context.Context, set component.ExporterCreateSettings, config config.Exporter) (component.LogsExporter, error) {
	expCfg := config.(*Config)

	if err := expCfg.validate(); err != nil {
		return nil, err
	}

	exp := newExporter(expCfg, set.Logger)

	return exporterhelper.NewLogsExporter(
		expCfg,
		set,
		exp.pushLogData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(expCfg.RetrySettings),
		exporterhelper.WithQueue(expCfg.QueueSettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.stop),
	)
}
