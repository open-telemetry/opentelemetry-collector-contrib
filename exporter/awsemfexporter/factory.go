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

package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

const (
	// The value of "type" key in configuration.
	typeStr = "awsemf"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for AWS EMF exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, stability))
}

// CreateDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() component.Config {
	return &Config{
		AWSSessionSettings:              awsutil.CreateDefaultSessionConfig(),
		LogGroupName:                    "",
		LogStreamName:                   "",
		Namespace:                       "",
		DimensionRollupOption:           "ZeroAndSingleDimensionRollup",
		RetainInitialValueOfDeltaMetric: false,
		OutputDestination:               "cloudwatch",
		logger:                          zap.NewNop(),
	}
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(_ context.Context,
	params exporter.CreateSettings,
	config component.Config) (exporter.Metrics, error) {

	expCfg := config.(*Config)

	return newEmfExporter(expCfg, params)
}
