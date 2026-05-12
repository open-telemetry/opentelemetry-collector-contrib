// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/metadata"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid openinference source",
			cfg: Config{
				Sources: []Source{{Name: SourceOpenInference, RemoveOriginals: true}},
			},
		},
		{
			name:    "no sources",
			cfg:     Config{Sources: []Source{}},
			wantErr: "at least one source",
		},
		{
			name: "unknown source",
			cfg: Config{
				Sources: []Source{{Name: "bogus"}},
			},
			wantErr: `sources[0]: unknown source "bogus"`,
		},
		{
			name: "duplicate source",
			cfg: Config{
				Sources: []Source{
					{Name: SourceOpenInference},
					{Name: SourceOpenInference},
				},
			},
			wantErr: `sources[1]: duplicate source "openinference"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

// TestDefaultConfigIsInvalid ensures the factory-returned default config fails
// validation, since the user must explicitly configure at least one source.
func TestDefaultConfigIsInvalid(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Empty(t, cfg.Sources)
	require.ErrorContains(t, cfg.Validate(), "at least one source must be specified")
}

// TestLoadConfig loads each named config from testdata/config.yaml, unmarshals
// it into a Config and either asserts the expected shape (for valid configs)
// or asserts the expected validation error message (for invalid configs).
func TestLoadConfig(t *testing.T) {
	tests := []struct {
		id           component.ID
		expected     *Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				Sources: []Source{
					{
						Name:            SourceOpenInference,
						RemoveOriginals: true,
						Overwrite:       false,
					},
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "empty"),
			errorMessage: "at least one source must be specified",
		},
		{
			id: component.NewIDWithName(metadata.Type, "openinference_only"),
			expected: &Config{
				Sources: []Source{{Name: SourceOpenInference}},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "empty_sources"),
			errorMessage: "at least one source must be specified",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "unknown_source"),
			errorMessage: `unknown source "foobar"`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "duplicate_source"),
			errorMessage: `duplicate source "openinference"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.errorMessage != "" {
				assert.ErrorContains(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			require.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
