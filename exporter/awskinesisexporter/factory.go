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

package awskinesisexporter

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr   = "awskinesis"
	// The default encoding scheme is set to otlpProto
	otlpProto = "otlp_proto"
	defaultEncoding = otlpProto
)

// NewFactory creates a factory for Kinesis exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTracesExporter),
		exporterhelper.WithMetrics(createMetricsExporter))
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
		AWS: AWSConfig{
			Region:     "us-west-2",
			StreamName: "test-stream",
		},
		KPL: KPLConfig{
			BatchSize:            5242880,
			BatchCount:           1000,
			BacklogCount:         2000,
			FlushIntervalSeconds: 5,
			MaxConnections:       24,
		},
		Encoding: defaultEncoding,
	}
}

func createTracesExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	config config.Exporter,
) (component.TracesExporter, error) {
	c := config.(*Config)
	return newExporter(c, params.Logger)
}

func createMetricsExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	config config.Exporter,
) (component.MetricsExporter, error) {
	c := config.(*Config)
	return newExporter(c, params.Logger)
}
