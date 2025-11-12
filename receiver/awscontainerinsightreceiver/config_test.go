// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/metadata"
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
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "collection_interval_settings"),
			expected: &Config{
				CollectionInterval:    60 * time.Second,
				ContainerOrchestrator: "eks",
				TagService:            true,
				PrefFullPodName:       false,
			},
		},
		// Test deprecated component name
		{
			id:       component.NewIDWithName(component.MustNewType("awscontainerinsightreceiver"), ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(component.MustNewType("awscontainerinsightreceiver"), "collection_interval_settings"),
			expected: &Config{
				CollectionInterval:    60 * time.Second,
				ContainerOrchestrator: "eks",
				TagService:            true,
				PrefFullPodName:       false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			var factory receiver.Factory
			if tt.id.Type() == metadata.Type {
				factory = NewFactory()
			} else {
				factory = NewDeprecatedFactory()
			}
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
