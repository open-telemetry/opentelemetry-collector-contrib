// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cumulativetodeltaprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				Include: MatchMetrics{
					Metrics: []string{
						"metric1",
						"metric2",
					},
					Config: filterset.Config{
						MatchType:    "strict",
						RegexpConfig: nil,
					},
				},
				Exclude: MatchMetrics{
					Metrics: []string{
						"metric3",
						"metric4",
					},
					Config: filterset.Config{
						MatchType:    "strict",
						RegexpConfig: nil,
					},
				},
				MaxStaleness: 10 * time.Second,
				InitialValue: tracking.InitialValueAuto,
			},
		},
		{
			id:       component.NewIDWithName(metadata.Type, "empty"),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "regexp"),
			expected: &Config{
				Include: MatchMetrics{
					Metrics: []string{
						"a*",
					},
					Config: filterset.Config{
						MatchType:    "regexp",
						RegexpConfig: nil,
					},
				},
				Exclude: MatchMetrics{
					Metrics: []string{
						"b*",
					},
					Config: filterset.Config{
						MatchType:    "regexp",
						RegexpConfig: nil,
					},
				},
				MaxStaleness: 10 * time.Second,
				InitialValue: tracking.InitialValueAuto,
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_match_type"),
			errorMessage: "match_type must be set if metrics are supplied",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_name"),
			errorMessage: "metrics must be supplied if match_type is set",
		},
		{
			id: component.NewIDWithName(metadata.Type, "auto"),
			expected: &Config{
				InitialValue: tracking.InitialValueAuto,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "keep"),
			expected: &Config{
				InitialValue: tracking.InitialValueKeep,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "drop"),
			expected: &Config{
				InitialValue: tracking.InitialValueDrop,
			},
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
