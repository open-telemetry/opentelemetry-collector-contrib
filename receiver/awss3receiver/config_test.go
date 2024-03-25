// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver/internal/metadata"
)

func TestLoadConfig_Validate_Invalid(t *testing.T) {
	cfg := Config{}
	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := Config{
		S3Downloader: S3DownloaderConfig{
			Region:           "",
			S3Bucket:         "abucket",
			S3Prefix:         "",
			S3Partition:      "",
			FilePrefix:       "",
			Endpoint:         "",
			S3ForcePathStyle: false,
		},
		StartTime: "2024-01-01",
		EndTime:   "2024-01-01",
	}
	assert.NoError(t, cfg.Validate())
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id:           component.NewIDWithName(metadata.Type, ""),
			errorMessage: "bucket is required; start time is required; end time is required",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "1"),
			errorMessage: "unable to parse start date; unable to parse end time",
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				S3Downloader: S3DownloaderConfig{
					Region:      "us-east-1",
					S3Bucket:    "abucket",
					S3Partition: "minute",
				},
				StartTime: "2024-01-31 15:00",
				EndTime:   "2024-02-03",
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

			if tt.errorMessage != "" {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
