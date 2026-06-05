// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Encoding = func() *component.ID {
					id := component.MustNewID("test")
					return &id
				}()
				cfg.Bucket.Name = "test-bucket"
				cfg.Bucket.Region = "test-region"
				cfg.Bucket.ProjectID = "test-project-id"
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_partition"),
			expected: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Encoding = func() *component.ID {
					id := component.MustNewID("test")
					return &id
				}()
				cfg.Bucket.Name = "test-bucket"
				cfg.Bucket.Region = "test-region"
				cfg.Bucket.ProjectID = "test-project-id"
				cfg.Bucket.Partition = partitionConfig{
					Format: "year=%Y",
					Prefix: "my-logs",
				}
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_resiliency"),
			expected: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Encoding = func() *component.ID {
					id := component.MustNewID("test")
					return &id
				}()
				cfg.TimeoutSettings.Timeout = 3 * time.Second
				cfg.QueueSettings = configoptional.Some(func() exporterhelper.QueueBatchConfig {
					queue := exporterhelper.NewDefaultQueueConfig()
					queue.NumConsumers = 7
					queue.QueueSize = 123
					return queue
				}())
				cfg.RetrySettings = configretry.NewDefaultBackOffConfig()
				cfg.RetrySettings.InitialInterval = time.Second
				cfg.RetrySettings.MaxInterval = 5 * time.Second
				cfg.RetrySettings.MaxElapsedTime = 20 * time.Second
				cfg.Bucket.Name = "test-bucket"
				cfg.Bucket.Region = "test-region"
				cfg.Bucket.ProjectID = "test-project-id"
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_gzip_compression"),
			expected: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Encoding = func() *component.ID {
					id := component.MustNewID("test")
					return &id
				}()
				cfg.Bucket.Name = "test-bucket"
				cfg.Bucket.Region = "test-region"
				cfg.Bucket.ProjectID = "test-project-id"
				cfg.Bucket.Compression = "gzip"
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_zstd_compression"),
			expected: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Encoding = func() *component.ID {
					id := component.MustNewID("test")
					return &id
				}()
				cfg.Bucket.Name = "test-bucket"
				cfg.Bucket.Region = "test-region"
				cfg.Bucket.ProjectID = "test-project-id"
				cfg.Bucket.Compression = "zstd"
				return cfg
			}(),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "empty_bucket_name"),
			expectedErr: errNameRequired,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_partition_format"),
			expectedErr: errFormatInvalid,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "unsupported_compression"),
			expectedErr: errUnknownCompression,
		},
	}

	for _, tt := range tests {
		name := strings.ReplaceAll(tt.id.String(), "/", "_")
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = xconfmap.Validate(cfg)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, cfg)
			}
		})
	}
}

func TestDefaultConfigQueueDisabled(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	cfg := NewFactory().CreateDefaultConfig()
	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	require.False(t, cfg.(*Config).QueueSettings.HasValue())
}
