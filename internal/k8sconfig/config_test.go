// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc        string
		cfg         APIConfig
		expectedErr string
	}{
		{
			desc: "valid service account auth",
			cfg:  APIConfig{AuthType: AuthTypeServiceAccount},
		},
		{
			desc: "valid none auth",
			cfg:  APIConfig{AuthType: AuthTypeNone},
		},
		{
			desc:        "invalid auth type",
			cfg:         APIConfig{AuthType: "invalid"},
			expectedErr: "invalid authType for kubernetes: invalid",
		},
		{
			desc: "zero qps and burst are valid (uses client-go defaults)",
			cfg:  APIConfig{AuthType: AuthTypeServiceAccount, KubeAPIQPS: 0, KubeAPIBurst: 0},
		},
		{
			desc: "positive qps and burst are valid",
			cfg:  APIConfig{AuthType: AuthTypeServiceAccount, KubeAPIQPS: 100, KubeAPIBurst: 200},
		},
		{
			desc:        "negative qps is invalid",
			cfg:         APIConfig{AuthType: AuthTypeServiceAccount, KubeAPIQPS: -1},
			expectedErr: "kube_api_qps must be greater than 0",
		},
		{
			desc:        "negative burst is invalid",
			cfg:         APIConfig{AuthType: AuthTypeServiceAccount, KubeAPIBurst: -1},
			expectedErr: "kube_api_burst must be greater than 0",
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

func TestCreateRestConfigAppliesRateLimits(t *testing.T) {
	// AuthTypeNone requires only KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT.
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	tests := []struct {
		desc          string
		cfg           APIConfig
		expectedQPS   float32
		expectedBurst int
	}{
		{
			desc:          "zero values do not override rest.Config (client-go applies its own defaults at request time)",
			cfg:           APIConfig{AuthType: AuthTypeNone, KubeAPIQPS: 0, KubeAPIBurst: 0},
			expectedQPS:   0,
			expectedBurst: 0,
		},
		{
			desc:          "custom qps and burst are written to rest.Config",
			cfg:           APIConfig{AuthType: AuthTypeNone, KubeAPIQPS: 100, KubeAPIBurst: 200},
			expectedQPS:   100,
			expectedBurst: 200,
		},
		{
			desc:          "only qps set, burst unchanged",
			cfg:           APIConfig{AuthType: AuthTypeNone, KubeAPIQPS: 50},
			expectedQPS:   50,
			expectedBurst: 0,
		},
		{
			desc:          "only burst set, qps unchanged",
			cfg:           APIConfig{AuthType: AuthTypeNone, KubeAPIBurst: 50},
			expectedQPS:   0,
			expectedBurst: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			rc, err := CreateRestConfig(tt.cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedQPS, rc.QPS)
			assert.Equal(t, tt.expectedBurst, rc.Burst)
		})
	}
}
