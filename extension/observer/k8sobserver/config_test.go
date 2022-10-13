// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sobserver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          config.ComponentID
		expected    config.Extension
		expectedErr string
	}{
		{
			id:       config.NewComponentID(typeStr),
			expected: NewFactory().CreateDefaultConfig(),
		},
		{
			id: config.NewComponentIDWithName(typeStr, "own-node-only"),
			expected: &Config{
				ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
				Node:              "node-1",
				APIConfig:         k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
				ObservePods:       true,
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "observe-all"),
			expected: &Config{
				ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
				Node:              "",
				APIConfig:         k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeNone},
				ObservePods:       true,
				ObserveNodes:      true,
			},
		},
		{
			id:          config.NewComponentIDWithName(typeStr, "invalid_auth"),
			expectedErr: "invalid authType for kubernetes: not a real auth type",
		},
		{
			id:          config.NewComponentIDWithName(typeStr, "invalid_no_observing"),
			expectedErr: "one of observe_pods and observe_nodes must be true",
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
			require.NoError(t, config.UnmarshalExtension(sub, cfg))
			if tt.expectedErr != "" {
				assert.EqualError(t, cfg.Validate(), tt.expectedErr)
				return
			}
			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
