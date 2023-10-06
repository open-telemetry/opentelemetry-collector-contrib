// SPDX-License-Identifier: Apache-2.0

package honeycombexporter

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter/internal/metadata"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	//defaultCfg := CreateDefaultConfig()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{

		{
			id: component.NewIDWithName("honeycomb", ""),
			expected: &Config{
				APIKey: "test-apikey",
				APIURL: "https://api.testhost.io",
				Markers: []marker{
					{
						MarkerType:   "fooType",
						MessageField: "test message",
						UrlField:     "https://api.testhost.io",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_log"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "unknown_log"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := CreateDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}
