// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecstaskobserver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
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
			id: config.NewComponentIDWithName(typeStr, "with-endpoint"),
			expected: &Config{
				ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://a.valid.url:1234/path",
				},
				PortLabels:      []string{"ECS_TASK_OBSERVER_PORT"},
				RefreshInterval: 100 * time.Second,
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "with-port-labels"),
			expected: &Config{
				ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
				PortLabels:        []string{"A_PORT_LABEL", "ANOTHER_PORT_LABEL"},
				RefreshInterval:   30 * time.Second,
			},
		},
		{
			id:          config.NewComponentIDWithName(typeStr, "invalid"),
			expectedErr: `failed to parse ecs task metadata endpoint "_:invalid": parse "_:invalid": first path segment in URL cannot contain colon`,
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
