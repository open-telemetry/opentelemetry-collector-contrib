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
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
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
func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		LogGroupName:          "",
		LogStreamName:         "",
		Namespace:             "",
		Endpoint:              "",
		RequestTimeoutSeconds: 30,
		MaxRetries:            1,
		NoVerifySSL:           false,
		ProxyAddress:          "",
		Region:                "",
		RoleARN:               "",
		DimensionRollupOption: "ZeroAndSingleDimensionRollup",
	}
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(_ context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter) (component.MetricsExporter, error) {

	expCfg := config.(*Config)

	return New(expCfg, params)
}
