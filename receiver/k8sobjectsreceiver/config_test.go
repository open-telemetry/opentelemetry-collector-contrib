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
		id       component.ID
		expected *Config
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
					},
					{
						Name:       "events",
						Mode:       WatchMode,
						Namespaces: []string{"default"},
						Group:      "events.k8s.io",
						ExcludeWatchType: []apiWatch.EventType{
							apiWatch.Deleted,
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
					},
					{
						Name:     "events",
						Mode:     PullMode,
						Interval: time.Hour,
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
					},
					{
						Name:            "events",
						Mode:            WatchMode,
						Namespaces:      []string{"default"},
						Group:           "events.k8s.io",
						ResourceVersion: "2",
					},
				},
			},
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

			assert.Equal(t, tt.expected.AuthType, cfg.AuthType)
			assert.Equal(t, tt.expected.Objects, cfg.Objects)

			err = cfg.Validate()
			if tt.expected == nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		desc        string
		cfg         *Config
		expectedErr string
	}{
		{
			desc: "invalid mode",
			cfg: &Config{
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
						Mode: "invalid_mode",
					},
				},
			},
			expectedErr: "invalid mode: invalid_mode",
		},
		{
			desc: "exclude watch type with pull mode",
			cfg: &Config{
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
						Mode: PullMode,
						ExcludeWatchType: []apiWatch.EventType{
							apiWatch.Deleted,
						},
					},
				},
			},
			expectedErr: "the Exclude config can only be used with watch mode",
		},
		{
			desc: "default mode is set",
			cfg: &Config{
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
					},
				},
			},
		},
		{
			desc: "default interval for pull mode",
			cfg: &Config{
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
						Mode: PullMode,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestDeepCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected *K8sObjectsConfig
	}{
		{
			name: "deep copy",
			expected: &K8sObjectsConfig{
				Name:             "pods",
				Group:            "group",
				Namespaces:       []string{"default"},
				Mode:             PullMode,
				FieldSelector:    "status.phase=Running",
				LabelSelector:    "environment in (production),tier in (frontend)",
				Interval:         time.Hour,
				ResourceVersion:  "1",
				ExcludeWatchType: []apiWatch.EventType{apiWatch.Added},
				exclude:          map[apiWatch.EventType]bool{apiWatch.Added: true},
				gvr: &schema.GroupVersionResource{
					Group:    "group",
					Version:  "v1",
					Resource: "pods",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.expected.DeepCopy()
			shallowCopy := tt.expected

			// Change all of the fields in the deep copy
			actual.Name = "changed"
			actual.Group = "changed"
			actual.Namespaces[0] = "changed"
			actual.Mode = WatchMode
			actual.FieldSelector = "changed"
			actual.LabelSelector = "changed"
			actual.Interval = time.Minute
			actual.ResourceVersion = "changed"
			actual.ExcludeWatchType[0] = apiWatch.Deleted
			actual.exclude[apiWatch.Bookmark] = true
			actual.gvr.Group = "changed"
			actual.gvr.Version = "changed"
			actual.gvr.Resource = "changed"

			// Make sure changing the deep copy didn't change the original value
			assert.Equal(t, tt.expected, shallowCopy)
		})
	}
}
