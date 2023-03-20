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

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "awss3"
	// The stability level of the exporter.
	stability = component.StabilityLevelAlpha
)

// NewFactory creates a factory for S3 exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
		exporter.WithLogs(createLogsExporter, stability))
}

func createDefaultConfig() component.Config {
	return &Config{
		S3Uploader: S3UploaderConfig{
			Region:      "us-east-1",
			S3Partition: "minute",
		},

		MarshalerName: "otlp_json",
		logger:        nil,
	}
}

func createLogsExporter(ctx context.Context,
	params exporter.CreateSettings,
	config component.Config) (exp exporter.Logs, err error) {

	s3Exporter, err := NewS3Exporter(config.(*Config), params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogsExporter(
		context.TODO(),
		params,
		config,
		s3Exporter.ConsumeLogs)
}

func createTracesExporter(ctx context.Context,
	params exporter.CreateSettings,
	config component.Config) (exp exporter.Traces, err error) {

	s3Exporter, err := NewS3Exporter(config.(*Config), params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(
		context.TODO(),
		params,
		config,
		s3Exporter.ConsumeTraces)
}
