// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter/internal/metadata"
)

func CreatePointer[T any](value T) *T {
	return &value
}

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
				Rotation: &ConfigRotation{
					MaxMegabytes: CreatePointer(10),
					MaxDays:      CreatePointer(3),
					MaxBackups:   CreatePointer(3),
					Localtime:    CreatePointer(true),
				},
				Format:        formatTypeJSON,
				FlushInterval: CreatePointer(time.Second),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "3"),
			expected: &Config{
				Path: "./filename",
				Rotation: &ConfigRotation{
					MaxMegabytes: CreatePointer(10),
					MaxDays:      CreatePointer(3),
					MaxBackups:   CreatePointer(3),
					Localtime:    CreatePointer(true),
				},
				Format:        formatTypeProto,
				Compression:   &compressionZSTD,
				FlushInterval: CreatePointer(time.Second),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "rotation_with_default_settings"),
			expected: &Config{
				Path:   "./foo",
				Format: formatTypeJSON,
				Rotation: &ConfigRotation{
					MaxBackups: CreatePointer(defaultMaxBackups),
				},
				FlushInterval: CreatePointer(time.Second),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "rotation_with_custom_settings"),
			expected: &Config{
				Path: "./foo",
				Rotation: &ConfigRotation{
					MaxMegabytes: CreatePointer(1234),
					MaxBackups:   CreatePointer(defaultMaxBackups),
				},
				Format:        formatTypeJSON,
				FlushInterval: CreatePointer(time.Second),
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "compression_error"),
			errorMessage: "invalid value (expected one of []interface {}{\"\", \"zstd\"}): \"gzip\"",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "format_error"),
			errorMessage: "invalid value (expected one of []interface {}{\"json\", \"proto\"}): \"text\"",
		},
		{
			id: component.NewIDWithName(metadata.Type, "flush_interval_5"),
			expected: &Config{
				Path:          "./flushed",
				FlushInterval: CreatePointer(time.Duration(5)),
				Format:        formatTypeJSON,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "flush_interval_5s"),
			expected: &Config{
				Path:          "./flushed",
				FlushInterval: CreatePointer(5 * time.Second),
				Format:        formatTypeJSON,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "flush_interval_500ms"),
			expected: &Config{
				Path:          "./flushed",
				FlushInterval: CreatePointer(500 * time.Millisecond),
				Format:        formatTypeJSON,
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
			dump(cfg)

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func dump(v any) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(jsonBytes))
}
