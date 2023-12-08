// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombmarkerexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName("honeycomb", ""),
			expected: &Config{
				APIKey: "test-apikey",
				APIURL: "https://api.testhost.io",
				Markers: []Marker{
					{
						Type:         "fooType",
						MessageField: "test message",
						URLField:     "https://api.testhost.io",
						Rules: Rules{
							ResourceConditions: []string{
								`IsMatch(attributes["test"], ".*")`,
							},
							LogConditions: []string{
								`body == "test"`,
							},
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName("honeycomb", "color_no_type"),
			expected: &Config{
				APIKey: "test-apikey",
				APIURL: "https://api.testhost.io",
				Markers: []Marker{
					{
						Color:        "green",
						MessageField: "test message",
						URLField:     "https://api.testhost.io",
						Rules: Rules{
							ResourceConditions: []string{
								`IsMatch(attributes["test"], ".*")`,
							},
							LogConditions: []string{
								`body == "test"`,
							},
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_log"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "no_conditions"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "no_api_key"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "no_markers_supplied"),
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
				err = component.ValidateConfig(cfg)
				assert.Error(t, err)
				return
			}

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := createDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}
