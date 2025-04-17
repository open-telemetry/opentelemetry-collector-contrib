// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiWatch "k8s.io/apimachinery/pkg/watch"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          component.ID
		expected    *Config
		expectedErr string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{
					AuthType: k8sconfig.AuthTypeServiceAccount,
				},
				Objects: []*K8sObjectsConfig{
					{
						Name:          "pods",
						Mode:          PullMode,
						Interval:      time.Hour,
						FieldSelector: "status.phase=Running",
						LabelSelector: "environment in (production),tier in (frontend)",
						exclude:       make(map[apiWatch.EventType]bool),
						gvr: &schema.GroupVersionResource{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
						},
					},
					{
						Name:       "events",
						Mode:       WatchMode,
						Namespaces: []string{"default"},
						Group:      "events.k8s.io",
						ExcludeWatchType: []apiWatch.EventType{
							apiWatch.Deleted,
						},
						exclude: map[apiWatch.EventType]bool{
							apiWatch.Deleted: true,
						},
						gvr: &schema.GroupVersionResource{
							Group:    "events.k8s.io",
							Version:  "v1",
							Resource: "events",
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "pull_with_resource"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{
					AuthType: k8sconfig.AuthTypeServiceAccount,
				},
				Objects: []*K8sObjectsConfig{
					{
						Name:            "pods",
						Mode:            PullMode,
						ResourceVersion: "1",
						Interval:        time.Hour,
						exclude:         make(map[apiWatch.EventType]bool),
						gvr: &schema.GroupVersionResource{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
						},
					},
					{
						Name:     "events",
						Mode:     PullMode,
						Interval: time.Hour,
						exclude:  make(map[apiWatch.EventType]bool),
						gvr: &schema.GroupVersionResource{
							Group:    "events.k8s.io",
							Version:  "v1",
							Resource: "events",
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "watch_with_resource"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{
					AuthType: k8sconfig.AuthTypeServiceAccount,
				},
				Objects: []*K8sObjectsConfig{
					{
						Name:            "events",
						Mode:            WatchMode,
						Namespaces:      []string{"default"},
						Group:           "events.k8s.io",
						ResourceVersion: "",
						exclude:         make(map[apiWatch.EventType]bool),
						gvr: &schema.GroupVersionResource{
							Group:    "events.k8s.io",
							Version:  "v1",
							Resource: "events",
						},
					},
					{
						Name:            "events",
						Mode:            WatchMode,
						Namespaces:      []string{"default"},
						Group:           "events.k8s.io",
						ResourceVersion: "2",
						exclude:         make(map[apiWatch.EventType]bool),
						gvr: &schema.GroupVersionResource{
							Group:    "events.k8s.io",
							Version:  "v1",
							Resource: "events",
						},
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_mode"),
			expectedErr: "invalid mode: invalid_mode",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "exclude_deleted_with_pull"),
			expectedErr: "the Exclude config can only be used with watch mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = cfg.Validate()
			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}

			// Initialize exclude maps and GVR after validation
			for i, obj := range cfg.Objects {
				obj.exclude = make(map[apiWatch.EventType]bool)
				for _, item := range obj.ExcludeWatchType {
					obj.exclude[item] = true
				}

				// Set GVR based on object name and group
				obj.gvr = &schema.GroupVersionResource{
					Group:    obj.Group,
					Version:  "v1",
					Resource: obj.Name,
				}
				if tt.expected != nil && i < len(tt.expected.Objects) {
					obj.gvr = tt.expected.Objects[i].gvr
				}
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected.AuthType, cfg.AuthType)
			assert.Equal(t, tt.expected.Objects, cfg.Objects)
		})
	}
}

func TestValidateResourceConflict(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)

	// Test default mode is set when not specified
	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name: "pods",
		},
	}

	err := rCfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, defaultMode, rCfg.Objects[0].Mode)
	assert.Equal(t, defaultPullInterval, rCfg.Objects[0].Interval)

	// Test pull mode with no interval gets default interval
	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name: "pods",
			Mode: PullMode,
		},
	}

	err = rCfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, defaultPullInterval, rCfg.Objects[0].Interval)

	// Test exclude watch type with pull mode returns error
	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name: "pods",
			Mode: PullMode,
			ExcludeWatchType: []apiWatch.EventType{
				apiWatch.Deleted,
			},
		},
	}

	err = rCfg.Validate()
	assert.EqualError(t, err, "the Exclude config can only be used with watch mode")
}
