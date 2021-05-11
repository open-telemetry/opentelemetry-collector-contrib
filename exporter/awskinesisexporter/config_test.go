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
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "default.yaml"), factories,
	)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters[config.NewID(typeStr)]

	assert.Equal(t, e,
		&Config{
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
		},
	)
}

func TestConfig(t *testing.T) {
	t.Parallel()

	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)
	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters[config.NewID(typeStr)]

	assert.Equal(t, e,
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
			AWS: AWSConfig{
				StreamName:      "test-stream",
				KinesisEndpoint: "awskinesis.mars-1.aws.galactic",
				Region:          "mars-1",
				Role:            "arn:test-role",
			},
			KPL: KPLConfig{
				AggregateBatchCount:  10,
				AggregateBatchSize:   11,
				BatchSize:            12,
				BatchCount:           13,
				BacklogCount:         14,
				FlushIntervalSeconds: 15,
				MaxConnections:       16,
				MaxRetries:           17,
				MaxBackoffSeconds:    18,
			},
			Encoding: "",
		},
	)
}

func TestConfigCheck(t *testing.T) {
	t.Parallel()
	cfg := (NewFactory()).CreateDefaultConfig()
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}
