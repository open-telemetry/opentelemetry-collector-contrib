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
	"go.opentelemetry.io/collector/filter"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiWatch "k8s.io/apimachinery/pkg/watch"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
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
						Mode:          k8sinventory.PullMode,
						Interval:      time.Hour,
						InitialDelay:  10 * time.Minute,
						FieldSelector: "status.phase=Running",
						LabelSelector: "environment in (production),tier in (frontend)",
					},
					{
						Name:       "events",
						Mode:       k8sinventory.WatchMode,
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
						Mode:            k8sinventory.PullMode,
						ResourceVersion: "1",
						Interval:        time.Hour,
						InitialDelay:    5 * time.Minute,
					},
					{
						Name:     "events",
						Mode:     k8sinventory.PullMode,
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
						Mode:            k8sinventory.WatchMode,
						Namespaces:      []string{"default"},
						Group:           "events.k8s.io",
						ResourceVersion: "",
					},
					{
						Name:            "events",
						Mode:            k8sinventory.WatchMode,
						Namespaces:      []string{"default"},
						Group:           "events.k8s.io",
						ResourceVersion: "2",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "top_level_interval"),
			expected: &Config{
				APIConfig: k8sconfig.APIConfig{
					AuthType: k8sconfig.AuthTypeServiceAccount,
				},
				Interval: 30 * time.Minute,
				Objects: []*K8sObjectsConfig{
					{
						Name:     "pods",
						Mode:     k8sinventory.PullMode,
						Interval: 30 * time.Minute, // resolved from top-level during Validate
					},
					{
						Name:     "nodes",
						Mode:     k8sinventory.PullMode,
						Interval: 5 * time.Minute, // per-resource override
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

			assert.Equal(t, tt.expected.APIConfig.AuthType, cfg.APIConfig.AuthType)

			err = cfg.Validate()
			if tt.expected == nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.Objects, cfg.Objects)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		desc               string
		cfg                *Config
		expectedErr        string
		expectedInterval   time.Duration
		expectZeroInterval bool
	}{
		{
			desc: "invalid mode",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
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
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
						Mode: k8sinventory.PullMode,
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
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
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
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
						Mode: k8sinventory.PullMode,
					},
				},
			},
		},
		{
			desc: "top-level interval is used when per-resource interval is unset",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Interval:  30 * time.Minute,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
						Mode: k8sinventory.PullMode,
					},
				},
			},
			expectedInterval: 30 * time.Minute,
		},
		{
			desc: "per-resource interval takes precedence over top-level interval",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Interval:  30 * time.Minute,
				Objects: []*K8sObjectsConfig{
					{
						Name:     "pods",
						Mode:     k8sinventory.PullMode,
						Interval: 5 * time.Minute,
					},
				},
			},
			expectedInterval: 5 * time.Minute,
		},
		{
			desc: "top-level interval has no effect on watch-mode objects",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Interval:  30 * time.Minute,
				Objects: []*K8sObjectsConfig{
					{
						Name: "events",
						Mode: k8sinventory.WatchMode,
					},
				},
			},
			expectZeroInterval: true,
		},
		{
			desc: "negative top-level interval is invalid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Interval:  -1 * time.Minute,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
						Mode: k8sinventory.PullMode,
					},
				},
			},
			expectedErr: "interval must not be negative",
		},
		{
			desc: "negative per-resource interval is invalid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name:     "pods",
						Mode:     k8sinventory.PullMode,
						Interval: -1 * time.Minute,
					},
				},
			},
			expectedErr: "objects[*].interval must not be negative",
		},
		{
			desc: "initial delay with watch mode is invalid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name:         "pods",
						Mode:         k8sinventory.WatchMode,
						InitialDelay: time.Minute,
					},
				},
			},
			expectedErr: "initial_delay can only be used with pull mode",
		},
		{
			desc: "negative initial delay is allowed",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name:         "pods",
						Mode:         k8sinventory.PullMode,
						InitialDelay: -time.Second,
					},
				},
			},
		},
		{
			desc: "initial delay equal to interval is invalid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name:         "pods",
						Mode:         k8sinventory.PullMode,
						Interval:     10 * time.Minute,
						InitialDelay: 10 * time.Minute,
					},
				},
			},
			expectedErr: "initial_delay must be less than interval",
		},
		{
			desc: "initial delay greater than top-level interval is invalid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Interval:  10 * time.Minute,
				Objects: []*K8sObjectsConfig{
					{
						Name:         "pods",
						Mode:         k8sinventory.PullMode,
						InitialDelay: 11 * time.Minute,
					},
				},
			},
			expectedErr: "initial_delay must be less than interval",
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
			if tt.expectedInterval != 0 {
				assert.Equal(t, tt.expectedInterval, tt.cfg.Objects[0].Interval)
			}
			if tt.expectZeroInterval {
				assert.Zero(t, tt.cfg.Objects[0].Interval)
			}
		})
	}
}

func TestLoadCustomResourcesConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	sub, err := cm.Sub("k8s_objects/custom_resources")
	require.NoError(t, err)

	cfg := createDefaultConfig().(*Config)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, cfg.Validate())
	assert.Equal(t, &CustomResourcesConfig{
		Interval:      15 * time.Minute,
		InitialDelay:  time.Minute,
		Namespaces:    []string{"default", "production"},
		LabelSelector: "app.kubernetes.io/managed-by=argocd",
		FieldSelector: "metadata.name!=example",
		Include: []CustomResourceSelector{
			{Group: "argoproj.io", Resources: []string{"applications"}},
			{Group: "opentelemetry.io", Resources: []string{"*"}},
		},
		Exclude: []CustomResourceSelector{
			{Group: "opentelemetry.io", Resources: []string{"instrumentations"}},
		},
	}, cfg.CustomResources)
}

func TestValidateCustomResources(t *testing.T) {
	t.Parallel()

	validInclude := []CustomResourceSelector{{Group: "*", Resources: []string{"*"}}}
	tests := []struct {
		name                 string
		interval             time.Duration
		customResources      CustomResourcesConfig
		expectedErr          string
		expectedPullInterval time.Duration
	}{
		{
			name: "uses default interval",
			customResources: CustomResourcesConfig{
				Include: validInclude,
			},
			expectedPullInterval: time.Hour,
		},
		{
			name:     "uses top-level interval",
			interval: 15 * time.Minute,
			customResources: CustomResourcesConfig{
				Include: validInclude,
			},
			expectedPullInterval: 15 * time.Minute,
		},
		{
			name:     "custom interval takes precedence",
			interval: 15 * time.Minute,
			customResources: CustomResourcesConfig{
				Interval: 10 * time.Minute,
				Include:  validInclude,
			},
			expectedPullInterval: 10 * time.Minute,
		},
		{
			name: "rejects negative interval",
			customResources: CustomResourcesConfig{
				Interval: -time.Minute,
				Include:  validInclude,
			},
			expectedErr: "custom_resources.interval must not be negative",
		},
		{
			name: "allows negative initial delay",
			customResources: CustomResourcesConfig{
				Interval:     10 * time.Minute,
				InitialDelay: -time.Minute,
				Include:      validInclude,
			},
			expectedPullInterval: 10 * time.Minute,
		},
		{
			name: "rejects initial delay equal to interval",
			customResources: CustomResourcesConfig{
				Interval:     10 * time.Minute,
				InitialDelay: 10 * time.Minute,
				Include:      validInclude,
			},
			expectedErr: "custom_resources.initial_delay must be less than interval",
		},
		{
			name: "rejects namespace allow and deny lists together",
			customResources: CustomResourcesConfig{
				Namespaces: []string{"default"},
				ExcludeNamespaces: []filter.Config{
					{Strict: "kube-system"},
				},
				Include: validInclude,
			},
			expectedErr: "custom_resources.namespaces and custom_resources.exclude_namespaces cannot both be set at the same time",
		},
		{
			name: "rejects invalid namespace filter",
			customResources: CustomResourcesConfig{
				ExcludeNamespaces: []filter.Config{
					{Regex: "["},
				},
				Include: validInclude,
			},
			expectedErr: "custom_resources.exclude_namespaces[0]: error parsing regexp: missing closing ]: `[`",
		},
		{
			name:        "rejects empty include",
			expectedErr: "custom_resources.include must not be empty",
		},
		{
			name: "rejects include without group",
			customResources: CustomResourcesConfig{
				Include: []CustomResourceSelector{{Resources: []string{"applications"}}},
			},
			expectedErr: "custom_resources.include[0].group must not be empty",
		},
		{
			name: "rejects include without resources",
			customResources: CustomResourcesConfig{
				Include: []CustomResourceSelector{{Group: "argoproj.io"}},
			},
			expectedErr: "custom_resources.include[0].resources must not be empty",
		},
		{
			name: "rejects empty exclude group",
			customResources: CustomResourcesConfig{
				Include: validInclude,
				Exclude: []CustomResourceSelector{{Resources: []string{"applications"}}},
			},
			expectedErr: "custom_resources.exclude[0].group must not be empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Interval = test.interval
			cfg.CustomResources = &test.customResources

			err := cfg.Validate()
			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expectedPullInterval, cfg.CustomResources.Interval)
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
				Mode:             k8sinventory.PullMode,
				FieldSelector:    "status.phase=Running",
				LabelSelector:    "environment in (production),tier in (frontend)",
				Interval:         time.Hour,
				InitialDelay:     10 * time.Minute,
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
			actual.Mode = k8sinventory.WatchMode
			actual.FieldSelector = "changed"
			actual.LabelSelector = "changed"
			actual.Interval = time.Minute
			actual.InitialDelay = time.Second
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

func TestCreateDefaultConfigIncludeInitialState(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	// Verify that IncludeInitialState defaults to nil/false
	assert.False(t, cfg.IncludeInitialState)
}

func TestConfigValidationIncludeInitialState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc        string
		cfg         *Config
		expectedErr string
	}{
		{
			desc: "include_initial_state true with watch mode is valid",
			cfg: &Config{
				APIConfig:           k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode:           PropagateError,
				IncludeInitialState: true,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
						Mode: k8sinventory.WatchMode,
					},
				},
			},
		},
		{
			desc: "include_initial_state false with watch mode is valid",
			cfg: &Config{
				APIConfig:           k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode:           PropagateError,
				IncludeInitialState: false,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
						Mode: k8sinventory.WatchMode,
					},
				},
			},
		},
		{
			desc: "include_initial_state true with pull mode is invalid",
			cfg: &Config{
				APIConfig:           k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode:           PropagateError,
				IncludeInitialState: true,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
						Mode: k8sinventory.PullMode,
					},
				},
			},
			expectedErr: "include_initial_state can only be used with watch mode",
		},
		{
			desc: "include_initial_state nil with watch mode is valid (defaults to false)",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name: "pods",
						Mode: k8sinventory.WatchMode,
					},
				},
			},
		},
		{
			desc: "namespaces and exclude_namespaces both set is invalid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
				ErrorMode: PropagateError,
				Objects: []*K8sObjectsConfig{
					{
						Name:       "pods",
						Mode:       k8sinventory.WatchMode,
						Namespaces: []string{"default"},
						ExcludeNamespaces: []filter.Config{{
							Regex: "namespace-to-ignore",
						}},
					},
				},
			},
			expectedErr: "namespaces and exclude_namespaces cannot both be set at the same time",
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

func TestConfigValidationAPIRateLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc          string
		cfg           *Config
		expectedErr   string
		expectedQPS   float32
		expectedBurst int
	}{
		{
			desc: "negative api_qps is invalid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount, KubeAPIQPS: -1, KubeAPIBurst: 10},
				ErrorMode: PropagateError,
				Objects:   []*K8sObjectsConfig{{Name: "pods", Mode: k8sinventory.WatchMode}},
			},
			expectedErr: "kube_api_qps must be greater than 0",
		},
		{
			desc: "negative api_burst is invalid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount, KubeAPIQPS: 5, KubeAPIBurst: -1},
				ErrorMode: PropagateError,
				Objects:   []*K8sObjectsConfig{{Name: "pods", Mode: k8sinventory.WatchMode}},
			},
			expectedErr: "kube_api_burst must be greater than 0",
		},
		{
			desc: "custom api_qps and api_burst are valid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount, KubeAPIQPS: 500, KubeAPIBurst: 1000},
				ErrorMode: PropagateError,
				Objects:   []*K8sObjectsConfig{{Name: "pods", Mode: k8sinventory.WatchMode}},
			},
			expectedQPS:   500,
			expectedBurst: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedQPS, tt.cfg.APIConfig.KubeAPIQPS)
			assert.Equal(t, tt.expectedBurst, tt.cfg.APIConfig.KubeAPIBurst)
		})
	}
}

func TestCreateDefaultConfigAPIRateLimit(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, k8sconfig.DefaultKubeAPIQPS, cfg.APIConfig.KubeAPIQPS)
	assert.Equal(t, k8sconfig.DefaultKubeAPIBurst, cfg.APIConfig.KubeAPIBurst)
}
