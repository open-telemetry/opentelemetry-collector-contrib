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
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 3, len(cfg.Exporters))

	r0 := cfg.Exporters[config.NewComponentID(typeStr)]
	assert.Equal(t, factory.CreateDefaultConfig(), r0)

	r1 := cfg.Exporters[config.NewComponentIDWithName(typeStr, "1")].(*Config)
	assert.NoError(t, r1.Validate())
	assert.Equal(t,
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "1")),
			AWSSessionSettings: awsutil.AWSSessionSettings{
				NumberOfWorkers:       8,
				Endpoint:              "",
				RequestTimeoutSeconds: 30,
				MaxRetries:            2,
				NoVerifySSL:           false,
				ProxyAddress:          "",
				Region:                "us-west-2",
				RoleARN:               "arn:aws:iam::123456789:role/monitoring-EKS-NodeInstanceRole",
			},
			LogGroupName:                    "",
			LogStreamName:                   "",
			DimensionRollupOption:           "ZeroAndSingleDimensionRollup",
			OutputDestination:               "cloudwatch",
			ParseJSONEncodedAttributeValues: make([]string, 0),
			MetricDeclarations:              []*MetricDeclaration{},
			MetricDescriptors:               []MetricDescriptor{},
		}, r1)

	r2 := cfg.Exporters[config.NewComponentIDWithName(typeStr, "resource_attr_to_label")].(*Config)
	assert.NoError(t, r2.Validate())
	assert.Equal(t, r2,
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "resource_attr_to_label")),
			AWSSessionSettings: awsutil.AWSSessionSettings{
				NumberOfWorkers:       8,
				Endpoint:              "",
				RequestTimeoutSeconds: 30,
				MaxRetries:            2,
				NoVerifySSL:           false,
				ProxyAddress:          "",
				Region:                "",
				RoleARN:               "",
			},
			LogGroupName:                    "",
			LogStreamName:                   "",
			DimensionRollupOption:           "ZeroAndSingleDimensionRollup",
			OutputDestination:               "cloudwatch",
			ResourceToTelemetrySettings:     resourcetotelemetry.Settings{Enabled: true},
			ParseJSONEncodedAttributeValues: make([]string, 0),
			MetricDeclarations:              []*MetricDeclaration{},
			MetricDescriptors:               []MetricDescriptor{},
		})
}

func TestConfigValidate(t *testing.T) {
	incorrectDescriptor := []MetricDescriptor{
		{metricName: ""},
		{unit: "Count", metricName: "apiserver_total", overwrite: true},
		{unit: "INVALID", metricName: "404"},
		{unit: "Megabytes", metricName: "memory_usage"},
	}
	cfg := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "1")),
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		MetricDescriptors:           incorrectDescriptor,
		logger:                      zap.NewNop(),
	}
	assert.NoError(t, cfg.Validate())

	assert.Equal(t, 2, len(cfg.MetricDescriptors))
	assert.Equal(t, []MetricDescriptor{
		{unit: "Count", metricName: "apiserver_total", overwrite: true},
		{unit: "Megabytes", metricName: "memory_usage"},
	}, cfg.MetricDescriptors)
}
