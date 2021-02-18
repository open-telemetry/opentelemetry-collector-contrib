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
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 3, len(cfg.Exporters))

	r0 := cfg.Exporters["awsemf"]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Exporters["awsemf/1"].(*Config)
	r1.Validate()
	assert.Equal(t,
		&Config{
			ExporterSettings:      configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "awsemf/1"},
			LogGroupName:          "",
			LogStreamName:         "",
			Endpoint:              "",
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
			NoVerifySSL:           false,
			ProxyAddress:          "",
			Region:                "us-west-2",
			RoleARN:               "arn:aws:iam::123456789:role/monitoring-EKS-NodeInstanceRole",
			DimensionRollupOption: "ZeroAndSingleDimensionRollup",
			MetricDeclarations:    []*MetricDeclaration{},
			MetricDescriptors:     []MetricDescriptor{},
		}, r1)

	r2 := cfg.Exporters["awsemf/resource_attr_to_label"].(*Config)
	r2.Validate()
	assert.Equal(t, r2,
		&Config{
			ExporterSettings:            configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "awsemf/resource_attr_to_label"},
			LogGroupName:                "",
			LogStreamName:               "",
			Endpoint:                    "",
			RequestTimeoutSeconds:       30,
			MaxRetries:                  1,
			NoVerifySSL:                 false,
			ProxyAddress:                "",
			Region:                      "",
			RoleARN:                     "",
			DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
			ResourceToTelemetrySettings: exporterhelper.ResourceToTelemetrySettings{Enabled: true},
			MetricDeclarations:          []*MetricDeclaration{},
			MetricDescriptors:           []MetricDescriptor{},
		})
}

func TestConfigValidate(t *testing.T) {
	incorrectDescriptor := []MetricDescriptor{
		{metricName: ""},
		{unit: "Count", metricName: "apiserver_total", overwrite: true},
		{unit: "INVALID", metricName: "404"},
		{unit: "Megabytes", metricName: "memory_usage"},
	}
	config := &Config{
		ExporterSettings:            configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "awsemf/resource_attr_to_label"},
		RequestTimeoutSeconds:       30,
		MaxRetries:                  1,
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		ResourceToTelemetrySettings: exporterhelper.ResourceToTelemetrySettings{Enabled: true},
		MetricDescriptors:           incorrectDescriptor,
		logger:                      zap.NewNop(),
	}
	config.Validate()

	assert.Equal(t, 2, len(config.MetricDescriptors))
	assert.Equal(t, []MetricDescriptor{
		{unit: "Count", metricName: "apiserver_total", overwrite: true},
		{unit: "Megabytes", metricName: "memory_usage"},
	}, config.MetricDescriptors)
}
