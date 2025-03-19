// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstarttimeprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/subtractinitial"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/truereset"
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
				Strategy:   truereset.Type,
				GCInterval: 10 * time.Minute,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "subtract_initial_point"),
			expected: &Config{
				Strategy:   subtractinitial.Type,
				GCInterval: 10 * time.Minute,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "gc_interval"),
			expected: &Config{
				Strategy:   truereset.Type,
				GCInterval: 1 * time.Hour,
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "negative_interval"),
			errorMessage: "gc_interval must be positive",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_strategy"),
			errorMessage: "\"bad\" is not a valid strategy",
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

			if tt.expected == nil {
				assert.EqualError(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
