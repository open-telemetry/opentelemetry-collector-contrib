// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				SamplingPercentage: 15.3,
				HashSeed:           22,
				AttributeSource:    "traceID",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "logs"),
			expected: &Config{
				SamplingPercentage: 15.3,
				HashSeed:           22,
				AttributeSource:    "record",
				FromAttribute:      "foo",
				SamplingPriority:   "bar",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			processors, err := cm.Sub("processors")
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := processors.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Processors[metadata.Type] = factory

	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "invalid.yaml"), factories)
	require.ErrorContains(t, err, "negative sampling rate: -15.30")
}
