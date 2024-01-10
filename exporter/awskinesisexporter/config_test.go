// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awskinesisexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "default"),
			expected: &Config{
				QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
				BackOffConfig:   configretry.NewDefaultBackOffConfig(),
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
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
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             false,
					MaxInterval:         30 * time.Second,
					InitialInterval:     5 * time.Second,
					MaxElapsedTime:      300 * time.Second,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigCheck(t *testing.T) {
	cfg := (NewFactory()).CreateDefaultConfig()
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}
