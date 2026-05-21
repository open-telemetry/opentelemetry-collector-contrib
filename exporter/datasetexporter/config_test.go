// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestConfigUnmarshalUnknownAttributes(t *testing.T) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	configMap := confmap.NewFromStringMap(map[string]any{
		"dataset_url":       "https://example.com",
		"api_key":           "secret",
		"unknown_attribute": "some value",
	})
	err := configMap.Unmarshal(config)

	assert.ErrorContains(t, err, "'datasetexporter.Config' has invalid keys: unknown_attribute")
}

func TestConfigUseDefaults(t *testing.T) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	configMap := confmap.NewFromStringMap(map[string]any{
		"dataset_url": "https://example.com",
		"api_key":     "secret",
	})
	err := config.Unmarshal(configMap)
	assert.NoError(t, err)

	assert.Equal(t, "https://example.com", config.DatasetURL)
	assert.Equal(t, "secret", string(config.APIKey))
	assert.Equal(t, bufferMaxLifetime, config.MaxLifetime)
	assert.Equal(t, logsExportResourceInfoDefault, config.ExportResourceInfo)
	assert.Equal(t, logsExportResourcePrefixDefault, config.ExportResourcePrefix)
	assert.Equal(t, logsExportScopeInfoDefault, config.ExportScopeInfo)
	assert.Equal(t, logsExportScopePrefixDefault, config.ExportScopePrefix)
	assert.Equal(t, logsDecomposeComplexMessageFieldDefault, config.DecomposeComplexMessageField)
	assert.Equal(t, exportSeparatorDefault, config.LogsSettings.ExportSeparator)
	assert.Equal(t, exportDistinguishingSuffix, config.LogsSettings.ExportDistinguishingSuffix)
	assert.Equal(t, exportSeparatorDefault, config.TracesSettings.ExportSeparator)
	assert.Equal(t, exportDistinguishingSuffix, config.TracesSettings.ExportDistinguishingSuffix)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected error
	}{
		{
			name: "valid config",
			config: Config{
				DatasetURL: "https://example.com",
				APIKey:     "secret",
				BufferSettings: BufferSettings{
					MaxLifetime: 123 * time.Millisecond,
				},
			},
			expected: nil,
		},
		{
			name: "missing api_key",
			config: Config{
				DatasetURL: "https://example.com",
				BufferSettings: BufferSettings{
					MaxLifetime: bufferMaxLifetime,
				},
			},
			expected: errors.New("api_key is required"),
		},
		{
			name: "missing dataset_url",
			config: Config{
				APIKey: "1234",
				BufferSettings: BufferSettings{
					MaxLifetime: bufferMaxLifetime,
				},
			},
			expected: errors.New("dataset_url is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil {
				assert.NoError(t, tt.expected, tt.name)
			} else {
				assert.Equal(t, tt.expected.Error(), err.Error(), tt.name)
			}
		})
	}
}

func TestConfigUseProvidedExportResourceInfoValue(t *testing.T) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	configMap := confmap.NewFromStringMap(map[string]any{
		"dataset_url": "https://example.com",
		"api_key":     "secret",
		"logs": map[string]any{
			"export_resource_info_on_event": true,
		},
	})
	err := config.Unmarshal(configMap)
	assert.NoError(t, err)
	assert.True(t, config.ExportResourceInfo)
}

func TestConfigUseProvidedExportScopeInfoValue(t *testing.T) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	configMap := confmap.NewFromStringMap(map[string]any{
		"dataset_url": "https://example.com",
		"api_key":     "secret",
		"logs": map[string]any{
			"export_scope_info_on_event": false,
		},
	})
	err := config.Unmarshal(configMap)
	assert.NoError(t, err)
	assert.False(t, config.ExportScopeInfo)
}

func TestConfigMarshalRoundTripPreservesExportSettings(t *testing.T) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	configMap := confmap.NewFromStringMap(map[string]any{
		"dataset_url": "https://example.com",
		"api_key":     "secret",
		"traces": map[string]any{
			"export_separator":             "::",
			"export_distinguishing_suffix": "__",
		},
		"logs": map[string]any{
			"export_scope_prefix":          "scope.",
			"export_separator":             "//",
			"export_distinguishing_suffix": "++",
		},
	})
	require.NoError(t, config.Unmarshal(configMap))

	out := confmap.New()
	require.NoError(t, out.Marshal(config))

	roundTrip := out.ToStringMap()
	assert.Equal(t, map[string]any{
		"export_separator":             "::",
		"export_distinguishing_suffix": "__",
	}, roundTrip["traces"])
	assert.Equal(t, map[string]any{
		"decompose_complex_message_field":   false,
		"decomposed_complex_message_prefix": "body.map.",
		"export_resource_info_on_event":     false,
		"export_resource_prefix":            "resource.attributes.",
		"export_scope_info_on_event":        true,
		"export_scope_prefix":               "scope.",
		"export_separator":                  "//",
		"export_distinguishing_suffix":      "++",
	}, roundTrip["logs"])
}
