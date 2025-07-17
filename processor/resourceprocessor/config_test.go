// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
		valid    bool
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				AttributesActions: []attraction.ActionKeyValue{
					{Key: "cloud.availability_zone", Value: "zone-1", Action: attraction.UPSERT},
					{Key: "k8s.cluster.name", FromAttribute: "k8s-cluster", Action: attraction.INSERT},
					{Key: "redundant-attribute", Action: attraction.DELETE},
				},
			},
			valid: true,
		},
		{
			id:       component.NewIDWithName(metadata.Type, "invalid"),
			expected: createDefaultConfig(),
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

			if tt.valid {
				assert.NoError(t, xconfmap.Validate(cfg))
			} else {
				assert.Error(t, xconfmap.Validate(cfg))
			}
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
