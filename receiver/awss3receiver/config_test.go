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
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver/internal/metadata"
)

func TestLoadConfig_Validate_Invalid(t *testing.T) {
	cfg := Config{}
	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_Valid(t *testing.T) {
	// Valid config with StartTime/EndTime
	t.Run("with time range", func(t *testing.T) {
		cfg := Config{
			S3Downloader: S3DownloaderConfig{
				Region:              "",
				S3Bucket:            "abucket",
				S3Prefix:            "",
				S3Partition:         "minute",
				FilePrefix:          "",
				Endpoint:            "",
				EndpointPartitionID: "aws",
				S3ForcePathStyle:    false,
			},
			StartTime: "2024-01-01",
			EndTime:   "2024-01-01",
		}
		assert.NoError(t, cfg.Validate())
	})

	// Valid config with SQS
	t.Run("with sqs", func(t *testing.T) {
		cfg := Config{
			S3Downloader: S3DownloaderConfig{
				Region:              "",
				S3Bucket:            "abucket",
				S3Prefix:            "",
				S3Partition:         "minute",
				FilePrefix:          "",
				Endpoint:            "",
				EndpointPartitionID: "aws",
				S3ForcePathStyle:    false,
			},
			SQS: &SQSConfig{
				QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
				Region:   "us-east-1",
			},
		}
		assert.NoError(t, cfg.Validate())
	})
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	opampExtension := component.NewIDWithName(component.MustNewType("opamp"), "bar")
	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id:           component.NewIDWithName(metadata.Type, ""),
			errorMessage: "bucket is required; either starttime/endtime or sqs configuration must be provided",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "1"),
			errorMessage: "s3_partition must be either 'hour' or 'minute'; unable to parse starttime (a date), accepted formats: 2006-01-02T15:04:05Z07:00, 2006-01-02 15:04, 2006-01-02; unable to parse endtime (2024-02-03a), accepted formats: 2006-01-02T15:04:05Z07:00, 2006-01-02 15:04, 2006-01-02",
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				S3Downloader: S3DownloaderConfig{
					Region:              "us-east-1",
					S3Bucket:            "abucket",
					S3Partition:         "minute",
					EndpointPartitionID: "aws",
				},
				StartTime: "2024-01-31 15:00",
				EndTime:   "2024-02-03",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "3"),
			expected: &Config{
				S3Downloader: S3DownloaderConfig{
					Region:              "us-east-1",
					S3Bucket:            "abucket",
					S3Partition:         "minute",
					EndpointPartitionID: "aws",
				},
				StartTime: "2024-01-31 15:00",
				EndTime:   "2024-02-03",
				Encodings: []Encoding{
					{
						Extension: component.NewIDWithName(component.MustNewType("foo"), "bar"),
						Suffix:    "baz",
					},
					{
						Extension: component.NewIDWithName(component.MustNewType("nop"), "nop"),
						Suffix:    "nop",
					},
				},
				Notifications: Notifications{
					OpAMP: &opampExtension,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "4"),
			expected: &Config{
				S3Downloader: S3DownloaderConfig{
					Region:              "us-east-1",
					S3Bucket:            "abucket",
					S3Partition:         "minute",
					EndpointPartitionID: "aws",
				},
				StartTime: "2024-01-31T15:00:00Z",
				EndTime:   "2024-02-03T00:00:00Z",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "5"),
			expected: &Config{
				S3Downloader: S3DownloaderConfig{
					Region:              "us-east-1",
					S3Bucket:            "abucket",
					S3Partition:         "minute",
					EndpointPartitionID: "aws",
				},
				SQS: &SQSConfig{
					QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
					Region:   "us-east-1",
					Endpoint: "http://localhost:4575",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.errorMessage != "" {
				assert.EqualError(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
