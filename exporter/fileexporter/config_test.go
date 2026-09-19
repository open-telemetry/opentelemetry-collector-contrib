// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				Path: "./filename.json",
				Rotation: &Rotation{
					MaxMegabytes: 10,
					MaxDays:      3,
					MaxBackups:   3,
					LocalTime:    true,
				},
				FormatType:            formatTypeJSON,
				FlushInterval:         time.Second,
				FilePermissions:       "0644",
				filePermissionsParsed: 0o644,
				GroupBy: &GroupBy{
					MaxOpenFiles:      defaultMaxOpenFiles,
					ResourceAttribute: defaultResourceAttribute,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "3"),
			expected: &Config{
				Path: "./filename",
				Rotation: &Rotation{
					MaxMegabytes: 10,
					MaxDays:      3,
					MaxBackups:   3,
					LocalTime:    true,
				},
				FormatType:            formatTypeProto,
				Compression:           compressionZSTD,
				FlushInterval:         time.Second,
				FilePermissions:       "0644",
				filePermissionsParsed: 0o644,
				GroupBy: &GroupBy{
					MaxOpenFiles:      defaultMaxOpenFiles,
					ResourceAttribute: defaultResourceAttribute,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "zstd_with_level"),
			expected: &Config{
				Path:        "./filename",
				FormatType:  formatTypeProto,
				Compression: compressionZSTD,
				CompressionParams: configcompression.CompressionParams{
					Level: 6,
				},
				FlushInterval:         time.Second,
				FilePermissions:       "0644",
				filePermissionsParsed: 0o644,
				GroupBy: &GroupBy{
					MaxOpenFiles:      defaultMaxOpenFiles,
					ResourceAttribute: defaultResourceAttribute,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "rotation_with_default_settings"),
			expected: &Config{
				Path:       "./foo",
				FormatType: formatTypeJSON,
				Rotation: &Rotation{
					MaxBackups: defaultMaxBackups,
				},
				FlushInterval:         time.Second,
				FilePermissions:       "0644",
				filePermissionsParsed: 0o644,
				GroupBy: &GroupBy{
					MaxOpenFiles:      defaultMaxOpenFiles,
					ResourceAttribute: defaultResourceAttribute,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "rotation_with_custom_settings"),
			expected: &Config{
				Path: "./foo",
				Rotation: &Rotation{
					MaxMegabytes: 1234,
					MaxBackups:   defaultMaxBackups,
				},
				FormatType:            formatTypeJSON,
				FlushInterval:         time.Second,
				FilePermissions:       "0644",
				filePermissionsParsed: 0o644,
				GroupBy: &GroupBy{
					MaxOpenFiles:      defaultMaxOpenFiles,
					ResourceAttribute: defaultResourceAttribute,
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "compression_error"),
			errorMessage: "compression is not supported",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "format_error"),
			errorMessage: "format type is not supported",
		},
		{
			id: component.NewIDWithName(metadata.Type, "flush_interval_5"),
			expected: &Config{
				Path:                  "./flushed",
				FlushInterval:         5,
				FormatType:            formatTypeJSON,
				FilePermissions:       "0644",
				filePermissionsParsed: 0o644,
				GroupBy: &GroupBy{
					MaxOpenFiles:      defaultMaxOpenFiles,
					ResourceAttribute: defaultResourceAttribute,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "flush_interval_5s"),
			expected: &Config{
				Path:                  "./flushed",
				FlushInterval:         5 * time.Second,
				FormatType:            formatTypeJSON,
				FilePermissions:       "0644",
				filePermissionsParsed: 0o644,
				GroupBy: &GroupBy{
					MaxOpenFiles:      defaultMaxOpenFiles,
					ResourceAttribute: defaultResourceAttribute,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "flush_interval_500ms"),
			expected: &Config{
				Path:                  "./flushed",
				FlushInterval:         500 * time.Millisecond,
				FormatType:            formatTypeJSON,
				FilePermissions:       "0644",
				filePermissionsParsed: 0o644,
				GroupBy: &GroupBy{
					MaxOpenFiles:      defaultMaxOpenFiles,
					ResourceAttribute: defaultResourceAttribute,
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "flush_interval_negative_value"),
			errorMessage: "flush_interval must be larger than zero",
		},
		{
			id:           component.NewIDWithName(metadata.Type, ""),
			errorMessage: "path must be non-empty",
		},
		{
			id: component.NewIDWithName(metadata.Type, "file_permissions"),
			expected: &Config{
				Path:                  "./filename",
				FormatType:            formatTypeJSON,
				FilePermissions:       "0600",
				filePermissionsParsed: 0o600,
				FlushInterval:         time.Second,
				GroupBy: &GroupBy{
					MaxOpenFiles:      defaultMaxOpenFiles,
					ResourceAttribute: defaultResourceAttribute,
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "file_permissions_invalid_octal"),
			errorMessage: "file_permissions value must be a valid octal representation",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "file_permissions_invalid_bits"),
			errorMessage: "file_permissions contain invalid bits for file access",
		},
		{
			id: component.NewIDWithName(metadata.Type, "group_by"),
			expected: &Config{
				Path:                  "./group_by/*.json",
				FlushInterval:         time.Second,
				FormatType:            formatTypeJSON,
				FilePermissions:       "0644",
				filePermissionsParsed: 0o644,
				GroupBy: &GroupBy{
					Enabled:           true,
					MaxOpenFiles:      10,
					ResourceAttribute: "dummy",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "group_by_defaults"),
			expected: &Config{
				Path:                  "./group_by/*.json",
				FlushInterval:         time.Second,
				FormatType:            formatTypeJSON,
				FilePermissions:       "0644",
				filePermissionsParsed: 0o644,
				GroupBy: &GroupBy{
					Enabled:           true,
					MaxOpenFiles:      defaultMaxOpenFiles,
					ResourceAttribute: defaultResourceAttribute,
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "group_by_invalid_path"),
			errorMessage: "path must contain exactly one * when group_by is enabled",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "group_by_invalid_path2"),
			errorMessage: "path must not start with * when group_by is enabled",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "group_by_empty_resource_attribute"),
			errorMessage: "resource_attribute must not be empty when group_by is enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				assert.EqualError(t, confmap.Validate(cfg), tt.errorMessage)
				return
			}

			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestDirectoryPermissionsWithoutCreateDirectory(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Path:                 "./foo",
		FormatType:           formatTypeJSON,
		CreateDirectory:      false,
		DirectoryPermissions: "0755",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory_permissions requires create_directory")
}

func TestFilePermissionsValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		filePermissions     string
		wantErr             error
		wantFilePermissions string
		wantParsed          int64
	}{
		{
			name:                "unset value defaults to 0644",
			wantFilePermissions: "0644",
			wantParsed:          0o644,
		},
		{
			name:                "valid value requires no other settings",
			filePermissions:     "0640",
			wantFilePermissions: "0640",
			wantParsed:          0o640,
		},
		{
			name:            "invalid octal value",
			filePermissions: "0999",
			wantErr:         errInvalidFilePermissionsOctal,
		},
		{
			name:            "value contains bits beyond file access bits",
			filePermissions: "7777",
			wantErr:         errInvalidFilePermissionsBits,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{
				Path:            "./foo",
				FormatType:      formatTypeJSON,
				FilePermissions: tt.filePermissions,
			}
			err := cfg.Validate()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantFilePermissions, cfg.FilePermissions)
			assert.Equal(t, tt.wantParsed, cfg.filePermissionsParsed)
		})
	}
}
