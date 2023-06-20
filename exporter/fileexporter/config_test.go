// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
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
				FormatType:    formatTypeJSON,
				FlushInterval: time.Second,
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
				FormatType:    formatTypeProto,
				Compression:   compressionZSTD,
				FlushInterval: time.Second,
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
				FlushInterval: time.Second,
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
				FormatType:    formatTypeJSON,
				FlushInterval: time.Second,
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
				Path:          "./flushed",
				FlushInterval: 5,
				FormatType:    formatTypeJSON,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "flush_interval_5s"),
			expected: &Config{
				Path:          "./flushed",
				FlushInterval: 5 * time.Second,
				FormatType:    formatTypeJSON,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "flush_interval_500ms"),
			expected: &Config{
				Path:          "./flushed",
				FlushInterval: 500 * time.Millisecond,
				FormatType:    formatTypeJSON,
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
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
