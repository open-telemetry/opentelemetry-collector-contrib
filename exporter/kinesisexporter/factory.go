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

package kinesisexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "kinesis"
)

// NewFactory creates a factory for Kinesis exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTraceExporter))
}

func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
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
	}
}

func createTraceExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter,
) (component.TracesExporter, error) {
	c := config.(*Config)
	exp, err := newExporter(c, params.Logger)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTraceExporter(
		c,
		params.Logger,
		exp.pushTraces,
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown))
}
