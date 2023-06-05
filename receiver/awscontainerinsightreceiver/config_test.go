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

package awscontainerinsightreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
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
			id:       component.NewIDWithName(typeStr, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(typeStr, "collection_interval_settings"),
			expected: &Config{
				CollectionInterval:        60 * time.Second,
				ContainerOrchestrator:     "eks",
				TagService:                true,
				PrefFullPodName:           false,
				LeaderLockName:            "otel-container-insight-clusterleader",
				EnableControlPlaneMetrics: false,
			},
		},
		{
			id: component.NewIDWithName(typeStr, "cluster_name"),
			expected: &Config{
				CollectionInterval:        60 * time.Second,
				ContainerOrchestrator:     "eks",
				TagService:                true,
				PrefFullPodName:           false,
				ClusterName:               "override_cluster",
				LeaderLockName:            "otel-container-insight-clusterleader",
				EnableControlPlaneMetrics: false,
			},
		},
		{
			id: component.NewIDWithName(typeStr, "leader_lock_name"),
			expected: &Config{
				CollectionInterval:        60 * time.Second,
				ContainerOrchestrator:     "eks",
				TagService:                true,
				PrefFullPodName:           false,
				LeaderLockName:            "override-container-insight-clusterleader",
				EnableControlPlaneMetrics: false,
			},
		},
		{
			id: component.NewIDWithName(typeStr, "leader_lock_using_config_map_only"),
			expected: &Config{
				CollectionInterval:           60 * time.Second,
				ContainerOrchestrator:        "eks",
				TagService:                   true,
				PrefFullPodName:              false,
				LeaderLockName:               "otel-container-insight-clusterleader",
				LeaderLockUsingConfigMapOnly: true,
				EnableControlPlaneMetrics:    false,
			},
		},
		{
			id: component.NewIDWithName(typeStr, "enable_control_plane_metrics"),
			expected: &Config{
				CollectionInterval:        60 * time.Second,
				ContainerOrchestrator:     "eks",
				TagService:                true,
				PrefFullPodName:           false,
				LeaderLockName:            "otel-container-insight-clusterleader",
				EnableControlPlaneMetrics: true,
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
