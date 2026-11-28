// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

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
				Region:                         "",
				S3Bucket:                       "abucket",
				S3Prefix:                       "",
				S3PartitionFormat:              "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				S3PartitionTimezone:            "UTC",
				FilePrefix:                     "",
				FilePrefixIncludeTelemetryType: true,
				Endpoint:                       "",
				EndpointPartitionID:            "aws",
				S3ForcePathStyle:               false,
				TagObjectAfterIngestion:        true,
				SkipIngestingTaggedObjects:     false,
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
				Region:                         "",
				S3Bucket:                       "abucket",
				S3Prefix:                       "",
				S3PartitionFormat:              "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				S3PartitionTimezone:            "UTC",
				FilePrefix:                     "",
				FilePrefixIncludeTelemetryType: true,
				Endpoint:                       "",
				EndpointPartitionID:            "aws",
				S3ForcePathStyle:               false,
				TagObjectAfterIngestion:        true,
				SkipIngestingTaggedObjects:     false,
			},
			SQS: &SQSConfig{
				QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
				Region:   "us-east-1",
			},
		}
		assert.NoError(t, cfg.Validate())
	})

	// Valid config with Poll
	t.Run("with poll", func(t *testing.T) {
		cfg := Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket:                   "abucket",
				TagObjectAfterIngestion:    true,
				SkipIngestingTaggedObjects: true,
			},
			Poll: &PollConfig{
				Interval: 5 * time.Minute,
				Lookback: 15 * time.Minute,
			},
		}
		assert.NoError(t, cfg.Validate())
	})

	// Valid config with Poll and StartTime for initial backfill
	t.Run("with poll and starttime", func(t *testing.T) {
		cfg := Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket:                   "abucket",
				TagObjectAfterIngestion:    true,
				SkipIngestingTaggedObjects: true,
			},
			StartTime: "2026-11-01 00:00",
			Poll: &PollConfig{
				Interval: 5 * time.Minute,
				Lookback: 15 * time.Minute,
			},
		}
		assert.NoError(t, cfg.Validate())
	})
}

func TestConfig_Validate_Poll(t *testing.T) {
	validPollCfg := func() Config {
		return Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket:                   "abucket",
				TagObjectAfterIngestion:    true,
				SkipIngestingTaggedObjects: true,
			},
			Poll: &PollConfig{
				Interval: 5 * time.Minute,
				Lookback: 10 * time.Minute,
			},
		}
	}

	t.Run("zero interval", func(t *testing.T) {
		cfg := validPollCfg()
		cfg.Poll.Interval = 0
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "poll.interval must be positive")
	})

	t.Run("negative interval", func(t *testing.T) {
		cfg := validPollCfg()
		cfg.Poll.Interval = -1 * time.Minute
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "poll.interval must be positive")
	})

	t.Run("negative lookback", func(t *testing.T) {
		cfg := validPollCfg()
		cfg.Poll.Lookback = -1 * time.Minute
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "poll.lookback cannot be negative")
	})

	t.Run("zero lookback is valid", func(t *testing.T) {
		cfg := validPollCfg()
		cfg.Poll.Lookback = 0
		assert.NoError(t, cfg.Validate())
	})

	t.Run("poll with endtime is rejected", func(t *testing.T) {
		cfg := validPollCfg()
		cfg.EndTime = "2026-11-06 12:00"
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "endtime cannot be configured when poll is enabled")
	})

	t.Run("poll with sqs is rejected", func(t *testing.T) {
		cfg := validPollCfg()
		cfg.SQS = &SQSConfig{
			QueueURL: "https://sqs.us-east-1.amazonaws.com/123/queue",
			Region:   "us-east-1",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "poll and sqs configuration cannot be used together")
	})

	t.Run("poll without tag_object_after_ingestion is rejected", func(t *testing.T) {
		cfg := validPollCfg()
		cfg.S3Downloader.TagObjectAfterIngestion = false
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "continuous polling requires both s3downloader.tag_object_after_ingestion and s3downloader.skip_ingesting_tagged_objects to be true")
	})

	t.Run("poll without skip_ingesting_tagged_objects is rejected", func(t *testing.T) {
		cfg := validPollCfg()
		cfg.S3Downloader.SkipIngestingTaggedObjects = false
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "continuous polling requires both s3downloader.tag_object_after_ingestion and s3downloader.skip_ingesting_tagged_objects to be true")
	})

	t.Run("poll with invalid starttime format is rejected", func(t *testing.T) {
		cfg := validPollCfg()
		cfg.StartTime = "invalid-date-format"
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to parse starttime")
	})

	t.Run("no mode configured error message includes poll", func(t *testing.T) {
		cfg := Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket: "abucket",
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "either starttime/endtime, sqs, or poll configuration must be provided")
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
			errorMessage: "bucket is required; either starttime/endtime, sqs, or poll configuration must be provided",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "1"),
			errorMessage: "unable to parse starttime (a date), accepted formats: 2006-01-02T15:04:05Z07:00, 2006-01-02 15:04, 2006-01-02; unable to parse endtime (2024-02-03a), accepted formats: 2006-01-02T15:04:05Z07:00, 2006-01-02 15:04, 2006-01-02",
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				S3Downloader: S3DownloaderConfig{
					Region:                         "us-east-1",
					S3Bucket:                       "abucket",
					S3PartitionFormat:              "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
					FilePrefixIncludeTelemetryType: true,
					EndpointPartitionID:            "aws",
					MaxUnmarshalSize:               defaultMaxUnmarshalSize,
					TagObjectAfterIngestion:        false,
					SkipIngestingTaggedObjects:     false,
				},
				StartTime: "2024-01-31 15:00",
				EndTime:   "2024-02-03",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "3"),
			expected: &Config{
				S3Downloader: S3DownloaderConfig{
					Region:                         "us-east-1",
					S3Bucket:                       "abucket",
					S3PartitionFormat:              "year=%Y/month=%m/day=%d/hour=%H",
					S3PartitionTimezone:            "Asia/Tokyo",
					FilePrefix:                     "otel",
					FilePrefixIncludeTelemetryType: false,
					EndpointPartitionID:            "aws",
					MaxUnmarshalSize:               defaultMaxUnmarshalSize,
					TagObjectAfterIngestion:        false,
					SkipIngestingTaggedObjects:     false,
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
					Region:                         "us-east-1",
					S3Bucket:                       "abucket",
					S3PartitionFormat:              "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
					FilePrefixIncludeTelemetryType: true,
					EndpointPartitionID:            "aws",
					MaxUnmarshalSize:               defaultMaxUnmarshalSize,
					TagObjectAfterIngestion:        true,
					SkipIngestingTaggedObjects:     true,
				},
				StartTime: "2024-01-31T15:00:00Z",
				EndTime:   "2024-02-03T00:00:00Z",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "5"),
			expected: &Config{
				S3Downloader: S3DownloaderConfig{
					Region:                         "us-east-1",
					S3Bucket:                       "abucket",
					S3PartitionFormat:              "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
					FilePrefixIncludeTelemetryType: true,
					EndpointPartitionID:            "aws",
					MaxUnmarshalSize:               defaultMaxUnmarshalSize,
					TagObjectAfterIngestion:        false,
					SkipIngestingTaggedObjects:     false,
				},
				SQS: &SQSConfig{
					QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
					Region:   "us-east-1",
					Endpoint: "http://localhost:4575",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "6"),
			expected: &Config{
				S3Downloader: S3DownloaderConfig{
					Region:                         "us-east-1",
					S3Bucket:                       "abucket",
					S3PartitionFormat:              "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
					FilePrefixIncludeTelemetryType: true,
					EndpointPartitionID:            "aws",
					MaxUnmarshalSize:               defaultMaxUnmarshalSize,
					Decompression:                  configcompression.TypeZstd,
				},
				StartTime: "2024-01-31 15:00",
				EndTime:   "2024-02-03",
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "7"),
			errorMessage: `unsupported decompression type "snappy", valid values are: gzip, zstd, none`,
		},
		{
			id: component.NewIDWithName(metadata.Type, "8"),
			expected: &Config{
				S3Downloader: S3DownloaderConfig{
					Region:                         "us-east-1",
					S3Bucket:                       "abucket",
					S3PartitionFormat:              "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
					FilePrefixIncludeTelemetryType: true,
					EndpointPartitionID:            "aws",
					MaxUnmarshalSize:               defaultMaxUnmarshalSize,
					TagObjectAfterIngestion:        true,
					SkipIngestingTaggedObjects:     true,
				},
				Poll: &PollConfig{
					Interval: 5 * time.Minute,
					Lookback: 30 * time.Minute,
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
				assert.EqualError(t, confmap.Validate(cfg), tt.errorMessage)
				return
			}

			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestMaxUnmarshalSizeConfig(t *testing.T) {
	tests := []struct {
		name    string
		size    int64
		wantErr bool
	}{
		{
			name: "default",
			size: 0,
		},
		{
			name: "positive",
			size: 1024,
		},
		{
			name:    "negative",
			size:    -1,
			wantErr: true,
		},
		{
			name:    "overflow",
			size:    math.MaxInt64,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			require.EqualValues(t, defaultMaxUnmarshalSize, cfg.S3Downloader.MaxUnmarshalSize)
			cm := confmap.NewFromStringMap(map[string]any{
				"s3downloader": map[string]any{
					"s3_bucket":          "bucket",
					"max_unmarshal_size": tt.size,
				},
				"starttime": "2024-01-01",
				"endtime":   "2024-01-02",
			})
			require.NoError(t, cm.Unmarshal(cfg))
			require.Equal(t, tt.size, cfg.S3Downloader.MaxUnmarshalSize)
			if tt.wantErr {
				require.ErrorContains(t, cfg.Validate(), "s3downloader.max_unmarshal_size")
			} else {
				require.NoError(t, cfg.Validate())
			}
		})
	}
}

func TestVisibilityTimeoutConfiguration(t *testing.T) {
	for _, value := range []int64{-1, 0, 1, 300, 43200, 43201} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cm := confmap.NewFromStringMap(map[string]any{
				"s3downloader": map[string]any{"s3_bucket": "bucket"},
				"sqs":          map[string]any{"queue_url": "queue", "region": "us-east-1", "visibility_timeout_seconds": value},
			})
			require.NoError(t, cm.Unmarshal(cfg))
			require.Equal(t, value, *cfg.SQS.VisibilityTimeoutSeconds)
			if value < 1 || value > 43200 {
				require.ErrorContains(t, cfg.Validate(), "sqs.visibility_timeout_seconds must be between 1 and 43200")
			} else {
				require.NoError(t, cfg.Validate())
			}
		})
	}
}
