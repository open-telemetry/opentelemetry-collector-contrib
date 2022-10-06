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
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected config.Exporter
	}{
		{
			id: config.NewComponentIDWithName(typeStr, "default"),
			expected: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
				QueueSettings:    exporterhelper.NewDefaultQueueSettings(),
				RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
				TimeoutSettings:  exporterhelper.NewDefaultTimeoutSettings(),
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
		},
		{
			id: config.NewComponentIDWithName(typeStr, ""),
			expected: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:         false,
					MaxInterval:     30 * time.Second,
					InitialInterval: 5 * time.Second,
					MaxElapsedTime:  300 * time.Second,
				},
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
				QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalExporter(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigCheck(t *testing.T) {
	cfg := (NewFactory()).CreateDefaultConfig()
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}
