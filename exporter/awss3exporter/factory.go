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

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/metadata"
)

// NewFactory creates a factory for S3 exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		S3Uploader: S3UploaderConfig{
			Region:      "us-east-1",
			S3Partition: "minute",
		},
		MarshalerName: "otlp_json",
	}
}

func createLogsExporter(ctx context.Context,
	params exporter.CreateSettings,
	config component.Config) (exporter.Logs, error) {

	s3Exporter, err := newS3Exporter(config.(*Config), params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogsExporter(ctx, params,
		config,
		s3Exporter.ConsumeLogs)
}

func createMetricsExporter(ctx context.Context,
	params exporter.CreateSettings,
	config component.Config) (exporter.Metrics, error) {

	s3Exporter, err := newS3Exporter(config.(*Config), params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(ctx, params,
		config,
		s3Exporter.ConsumeMetrics)
}

func createTracesExporter(ctx context.Context,
	params exporter.CreateSettings,
	config component.Config) (exporter.Traces, error) {

	s3Exporter, err := newS3Exporter(config.(*Config), params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(ctx,
		params,
		config,
		s3Exporter.ConsumeTraces)
}
