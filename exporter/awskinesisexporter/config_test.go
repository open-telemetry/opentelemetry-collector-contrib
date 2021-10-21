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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
)

func TestDefaultConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "default.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters[config.NewComponentID(typeStr)]

	assert.Equal(t, e,
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
			QueueSettings:    exporterhelper.DefaultQueueSettings(),
			RetrySettings:    exporterhelper.DefaultRetrySettings(),
			TimeoutSettings:  exporterhelper.DefaultTimeoutSettings(),
			Encoding: Encoding{
				Name:        "otlp",
				Compression: "none",
			},
			AWS: AWSConfig{
				Region: "us-west-2",
			},
			MaxRecordsPerBatch: batch.MaxBatchedRecords,
			MaxRecordSize:      batch.MaxRecordSize,
		},
	)
}

func TestConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters[config.NewComponentID(typeStr)]

	assert.Equal(t, e,
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
			RetrySettings: exporterhelper.RetrySettings{
				Enabled:         false,
				MaxInterval:     30 * time.Second,
				InitialInterval: 5 * time.Second,
				MaxElapsedTime:  300 * time.Second,
			},
			TimeoutSettings: exporterhelper.DefaultTimeoutSettings(),
			QueueSettings:   exporterhelper.DefaultQueueSettings(),
			Encoding: Encoding{
				Name:        "otlp-proto",
				Compression: "none",
			},
			AWS: AWSConfig{
				StreamName:      "test-stream",
				KinesisEndpoint: "awskinesis.mars-1.aws.galactic",
				Region:          "mars-1",
				Role:            "arn:test-role",
			},
			MaxRecordSize:      1000,
			MaxRecordsPerBatch: 10,
		},
	)
}

func TestConfigCheck(t *testing.T) {
	cfg := (NewFactory()).CreateDefaultConfig()
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}
