// Copyright 2020, OpenTelemetry Authors
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

package elasticexporter

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "elastic"

	mappingFieldName = "mapping"
)

// NewFactory creates a factory for Elastic exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTraceExporter),
		exporterhelper.WithMetrics(createMetricsExporter),
		// TODO: hook up 'createLogsExporter'
	)
}

func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		ElasticsearchConfig: ElasticsearchConfig{
			Timeout: 5 * time.Minute,
			Index:   "logs-generic",
			Retry: RetryConfig{
				Enabled:         true,
				Max:             3,
				InitialInterval: 100 * time.Millisecond,
				MaxInterval:     1 * time.Minute,
			},
		},
	}
}

func createTraceExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.TracesExporter, error) {
	return newTraceExporter(params, cfg)
}

func createMetricsExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.MetricsExporter, error) {
	return newMetricsExporter(params, cfg)
}

func createLogsExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.LogsExporter, error) {
	return newLogsExporter(params, cfg)
}
