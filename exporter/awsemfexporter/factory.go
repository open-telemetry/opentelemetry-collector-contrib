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

package awsemfexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

const (
	// The value of "type" key in configuration.
	typeStr = "awsemf"
)

// NewFactory creates a factory for AWS EMF exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithMetrics(createMetricsExporter))
}

// CreateDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings:                config.NewExporterSettings(config.NewComponentID(typeStr)),
		AWSSessionSettings:              awsutil.CreateDefaultSessionConfig(),
		LogGroupName:                    "",
		LogStreamName:                   "",
		Namespace:                       "",
		DimensionRollupOption:           "ZeroAndSingleDimensionRollup",
		ParseJSONEncodedAttributeValues: make([]string, 0),
		MetricDeclarations:              make([]*MetricDeclaration, 0),
		MetricDescriptors:               make([]MetricDescriptor, 0),
		OutputDestination:               "cloudwatch",
		logger:                          nil,
	}
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(_ context.Context,
	params component.ExporterCreateSettings,
	config config.Exporter) (component.MetricsExporter, error) {

	expCfg := config.(*Config)

	return newEmfExporter(expCfg, params)
}
