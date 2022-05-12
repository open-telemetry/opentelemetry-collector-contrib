// Copyright 2022 OpenTelemetry Authors
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

package awss3exporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr       = "awss3"
)

// NewFactory creates a factory for S3 exporter.
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsExporter(createMetricsExporter),
		component.WithLogsExporter(createLogsExporter),
		component.WithTracesExporter(createTracesExporter))
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),

		S3Uploader: S3UploaderConfig{
			Region:              "us-east-1",
			S3Partition:         "minute",
		},

		MetricDescriptors: make([]MetricDescriptor, 0),
		logger:            nil,
	}
}

func createMetricsExporter(ctx context.Context,
	params component.ExporterCreateSettings,
	config config.Exporter) (component.MetricsExporter, error) {

	s3Exporter, err := NewS3Exporter(config, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(
		config,
		params,
		s3Exporter.ConsumeMetrics)
}

func createLogsExporter(ctx context.Context,
	params component.ExporterCreateSettings,
	config config.Exporter) (component.LogsExporter, error) {

	s3Exporter, err := NewS3Exporter(config, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogsExporter(
		config,
		params,
		s3Exporter.ConsumeLogs)
}

func createTracesExporter(ctx context.Context,
	params component.ExporterCreateSettings,
	config config.Exporter) (component.TracesExporter, error) {

	s3Exporter, err := NewS3Exporter(config, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(
		config,
		params,
		s3Exporter.ConsumeTraces)
}
