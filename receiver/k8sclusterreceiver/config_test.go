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

package k8sclusterreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id:       component.NewIDWithName(typeStr, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(typeStr, "all_settings"),
			expected: &Config{
				Distribution:               distributionKubernetes,
				CollectionInterval:         30 * time.Second,
				NodeConditionTypesToReport: []string{"Ready", "MemoryPressure"},
				AllocatableTypesToReport:   []string{"cpu", "memory"},
				MetadataExporters:          []string{"nop"},
				APIConfig: k8sconfig.APIConfig{
					AuthType: k8sconfig.AuthTypeServiceAccount,
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "partial_settings"),
			expected: &Config{
				Distribution:               distributionOpenShift,
				CollectionInterval:         30 * time.Second,
				NodeConditionTypesToReport: []string{"Ready"},
				APIConfig: k8sconfig.APIConfig{
					AuthType: k8sconfig.AuthTypeServiceAccount,
				},
			},
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

func TestInvalidConfig(t *testing.T) {
	// No APIConfig
	cfg := &Config{
		Distribution:       distributionKubernetes,
		CollectionInterval: 30 * time.Second,
	}
	err := component.ValidateConfig(cfg)
	assert.NotNil(t, err)
	assert.Equal(t, "invalid authType for kubernetes: ", err.Error())

	// Wrong distro
	cfg = &Config{
		APIConfig:          k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeNone},
		Distribution:       "wrong",
		CollectionInterval: 30 * time.Second,
	}
	err = component.ValidateConfig(cfg)
	assert.NotNil(t, err)
	assert.Equal(t, "\"wrong\" is not a supported distribution. Must be one of: \"openshift\", \"kubernetes\"", err.Error())
}
