// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter/internal/metadata"
)

func TestValidate(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				Encoding: func() *component.ID {
					id := component.MustNewID("test")
					return &id
				}(),
				Bucket: bucketConfig{
					Name:       "test-bucket",
					Region:     "test-region",
					ProjectID:  "test-project-id",
					FilePrefix: "logs",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_partition"),
			expected: &Config{
				Encoding: func() *component.ID {
					id := component.MustNewID("test")
					return &id
				}(),
				Bucket: bucketConfig{
					Name:       "test-bucket",
					Region:     "test-region",
					ProjectID:  "test-project-id",
					FilePrefix: "logs",
					Partition: partitionConfig{
						Format: "year=%Y",
						Prefix: "my-logs",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_gzip_compression"),
			expected: &Config{
				Encoding: func() *component.ID {
					id := component.MustNewID("test")
					return &id
				}(),
				Bucket: bucketConfig{
					Name:        "test-bucket",
					Region:      "test-region",
					ProjectID:   "test-project-id",
					FilePrefix:  "logs",
					Compression: "gzip",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_zstd_compression"),
			expected: &Config{
				Encoding: func() *component.ID {
					id := component.MustNewID("test")
					return &id
				}(),
				Bucket: bucketConfig{
					Name:        "test-bucket",
					Region:      "test-region",
					ProjectID:   "test-project-id",
					FilePrefix:  "logs",
					Compression: "zstd",
				},
			},
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
