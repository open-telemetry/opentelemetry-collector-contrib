// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/metadata"
)

func TestConfigValidationAPIRateLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc        string
		cfg         *Config
		expectedErr string
	}{
		{
			desc: "negative qps is invalid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount, KubeAPIQPS: -1, KubeAPIBurst: 10},
			},
			expectedErr: "kube_api_qps must be greater than 0",
		},
		{
			desc: "negative burst is invalid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount, KubeAPIQPS: 5, KubeAPIBurst: -1},
			},
			expectedErr: "kube_api_burst must be greater than 0",
		},
		{
			desc: "custom qps and burst are valid",
			cfg: &Config{
				APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount, KubeAPIQPS: 100, KubeAPIBurst: 200},
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
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "all_settings"),
			expected: &Config{
				Namespaces: []string{"default", "my_namespace"},
				APIConfig: k8sconfig.APIConfig{
					AuthType:     k8sconfig.AuthTypeServiceAccount,
					KubeAPIQPS:   k8sconfig.DefaultKubeAPIQPS,
					KubeAPIBurst: k8sconfig.DefaultKubeAPIBurst,
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
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
