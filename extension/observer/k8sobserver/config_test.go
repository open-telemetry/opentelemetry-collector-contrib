// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: NewFactory().CreateDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "own-node-only"),
			expected: &Config{
				Node:        "node-1",
				APIConfig:   k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
				ObservePods: true,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "observe-all"),
			expected: &Config{
				Node:            "",
				APIConfig:       k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeNone},
				ObservePods:     true,
				ObserveNodes:    true,
				ObserveServices: true,
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_auth"),
			expectedErr: "invalid authType for kubernetes: not a real auth type",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_no_observing"),
			expectedErr: "one of observe_pods, observe_nodes and observe_services must be true",
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
			if tt.expectedErr != "" {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.expectedErr)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
