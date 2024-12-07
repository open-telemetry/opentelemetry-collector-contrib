// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cgroupruntimeextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id                    component.ID
		expected              component.Config
		unmarshalErrorMessage string
		validateErrorMessage  string
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				GoMaxProcs: GoMaxProcsConfig{Enabled: true},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.9,
				},
			},
		},
		{
			id:                   component.NewIDWithName(metadata.Type, "invalid_ratio"),
			validateErrorMessage: "gomemlimit ratio must be in the (0.0,1.0] range",
		},
		{
			id:                   component.NewIDWithName(metadata.Type, "invalid_ratio_disabled"),
			validateErrorMessage: "gomemlimit ratio must be in the (0.0,1.0] range",
		},
		{
			id:                   component.NewIDWithName(metadata.Type, "invalid_ratio_negative"),
			validateErrorMessage: "gomemlimit ratio must be in the (0.0,1.0] range",
		},
		{
			id:                    component.NewIDWithName(metadata.Type, "invalid_ratio_type"),
			unmarshalErrorMessage: "decoding failed due to the following error(s):\n\n'gomemlimit.ratio' expected type 'float64', got unconvertible type 'string', value: 'not_valid'",
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

			if tt.unmarshalErrorMessage != "" {
				assert.ErrorContains(t, sub.Unmarshal(cfg), tt.unmarshalErrorMessage)
				return
			}
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.validateErrorMessage != "" {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.validateErrorMessage)
				return
			}

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
