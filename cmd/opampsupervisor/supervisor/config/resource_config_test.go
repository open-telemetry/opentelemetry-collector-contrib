// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	otelconftelemetry "go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"
	otelconf "go.opentelemetry.io/contrib/otelconf/v0.3.0"
)

func TestTelemetryResourceConfigUnmarshal(t *testing.T) {
	t.Run("legacy inline attributes", func(t *testing.T) {
		conf := confmap.NewFromStringMap(map[string]any{
			"service.name": "my-service",
			"custom.attr":  "value",
		})

		var cfg otelconftelemetry.ResourceConfig
		require.NoError(t, conf.Unmarshal(&cfg))
		assert.Empty(t, cfg.Attributes)
		assert.Equal(t, "my-service", cfg.LegacyAttributes["service.name"])
		assert.Equal(t, "value", cfg.LegacyAttributes["custom.attr"])
	})

	t.Run("declarative resource accepts detectors", func(t *testing.T) {
		conf := confmap.NewFromStringMap(map[string]any{
			"schema_url": "https://opentelemetry.io/schemas/1.38.0",
			"attributes": []any{
				map[string]any{"name": "service.name", "value": "svc"},
			},
			"detectors": map[string]any{
				"attributes": map[string]any{
					"included": []any{"host"},
				},
			},
		})

		var cfg otelconftelemetry.ResourceConfig
		require.NoError(t, conf.Unmarshal(&cfg))
		require.NoError(t, xconfmap.Validate(&cfg))
		require.NotNil(t, cfg.SchemaUrl)
		assert.Equal(t, "https://opentelemetry.io/schemas/1.38.0", *cfg.SchemaUrl)
		assert.Len(t, cfg.Attributes, 1)
		require.NotNil(t, cfg.Detectors)
	})

	t.Run("attributes list rejected", func(t *testing.T) {
		conf := confmap.NewFromStringMap(map[string]any{
			"attributes_list": "ignored",
		})

		var cfg otelconftelemetry.ResourceConfig
		require.NoError(t, conf.Unmarshal(&cfg))
		require.ErrorContains(t, xconfmap.Validate(&cfg), "resource::attributes_list is not currently supported")
	})

	t.Run("legacy and declarative attributes cannot be mixed", func(t *testing.T) {
		conf := confmap.NewFromStringMap(map[string]any{
			"attributes": []any{
				map[string]any{"name": "service.name", "value": "svc"},
			},
			"legacy.attr": "value",
		})

		var cfg otelconftelemetry.ResourceConfig
		require.NoError(t, conf.Unmarshal(&cfg))
		require.ErrorContains(t, xconfmap.Validate(&cfg), "resource::attributes cannot be used together with legacy inline resource attributes")
	})
}

func TestTelemetryResourceConfigMarshal(t *testing.T) {
	schemaURL := "https://opentelemetry.io/schemas/1.38.0"
	cfg := otelconftelemetry.ResourceConfig{
		Resource: otelconf.Resource{
			SchemaUrl: &schemaURL,
			Attributes: []otelconf.AttributeNameValue{
				{Name: "service.name", Value: "custom-service"},
			},
		},
		LegacyAttributes: map[string]any{
			"legacy.attr":     "legacy-value",
			"service.version": nil,
		},
	}

	cm := confmap.New()
	require.NoError(t, cm.Marshal(cfg))
	raw := cm.ToStringMap()

	assert.Equal(t, "legacy-value", raw["legacy.attr"])
	assert.Contains(t, raw, "service.version")
	assert.Nil(t, raw["service.version"])
	assert.Equal(t, "https://opentelemetry.io/schemas/1.38.0", raw["schema_url"])

	attrs, ok := raw["attributes"].([]any)
	require.True(t, ok)
	require.Len(t, attrs, 1)

	attr, ok := attrs[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "service.name", attr["name"])
	assert.Equal(t, "custom-service", attr["value"])
}

func TestLoadRejectsInvalidTelemetryResourceConfig(t *testing.T) {
	tmpDir := t.TempDir()
	executablePath := filepath.Join(tmpDir, "binary")
	require.NoError(t, os.WriteFile(executablePath, []byte{}, 0o600))

	cfgPath := setupSupervisorConfigFile(t, tmpDir, fmt.Sprintf(`
server:
  endpoint: ws://localhost/v1/opamp

agent:
  executable: %s

telemetry:
  resource:
    attributes_list: unsupported
`, executablePath))

	_, err := Load(cfgPath)
	require.ErrorContains(t, err, "invalid telemetry::resource settings")
	require.ErrorContains(t, err, "resource::attributes_list is not currently supported")
}
