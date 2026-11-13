// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sleaderelector

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id             component.ID
		expectedConfig component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "defaults"),
			expectedConfig: &Config{
				APIConfig: k8sconfig.APIConfig{
					AuthType: "kubeConfig",
				},
				LeaseName:      "foo",
				LeaseNamespace: "default",
				LeaseDuration:  15 * time.Second,
				RenewDuration:  10 * time.Second,
				RetryPeriod:    2 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_lease_duration"),
			expectedConfig: &Config{
				APIConfig: k8sconfig.APIConfig{
					AuthType: "kubeConfig",
				},
				LeaseName:      "bar",
				LeaseNamespace: "default",
				LeaseDuration:  20 * time.Second,
				RenewDuration:  10 * time.Second,
				RetryPeriod:    2 * time.Second,
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

			require.Equal(t, tt.expectedConfig, cfg)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name         string
		modifyConfig func(*Config)
		expectedErr  string
	}{
		{
			name: "valid config",
		},
		{
			name: "missing lease name",
			modifyConfig: func(cfg *Config) {
				cfg.LeaseName = ""
			},
			expectedErr: "lease name and namespace must be set",
		},
		{
			name: "missing lease namespace",
			modifyConfig: func(cfg *Config) {
				cfg.LeaseNamespace = ""
			},
			expectedErr: "lease name and namespace must be set",
		},
		{
			name: "missing both lease name and namespace",
			modifyConfig: func(cfg *Config) {
				cfg.LeaseName = ""
				cfg.LeaseNamespace = ""
			},
			expectedErr: "lease name and namespace must be set",
		},
		{
			name: "zero lease duration",
			modifyConfig: func(cfg *Config) {
				cfg.LeaseDuration = 0
			},
			expectedErr: "lease_duration must be greater than zero",
		},
		{
			name: "negative lease duration",
			modifyConfig: func(cfg *Config) {
				cfg.LeaseDuration = -time.Second
			},
			expectedErr: "lease_duration must be greater than zero",
		},
		{
			name: "zero renew deadline",
			modifyConfig: func(cfg *Config) {
				cfg.RenewDuration = 0
			},
			expectedErr: "renew_deadline must be greater than zero",
		},
		{
			name: "negative renew deadline",
			modifyConfig: func(cfg *Config) {
				cfg.RenewDuration = -time.Second
			},
			expectedErr: "renew_deadline must be greater than zero",
		},
		{
			name: "zero retry period",
			modifyConfig: func(cfg *Config) {
				cfg.RetryPeriod = 0
			},
			expectedErr: "retry_period must be greater than zero",
		},
		{
			name: "negative retry period",
			modifyConfig: func(cfg *Config) {
				cfg.RetryPeriod = -time.Second
			},
			expectedErr: "retry_period must be greater than zero",
		},
		{
			name: "lease duration equals renew deadline",
			modifyConfig: func(cfg *Config) {
				cfg.LeaseDuration = cfg.RenewDuration
			},
			expectedErr: "lease_duration must be greater than renew_deadline",
		},
		{
			name: "lease duration less than renew deadline",
			modifyConfig: func(cfg *Config) {
				cfg.LeaseDuration = cfg.RenewDuration - time.Second
			},
			expectedErr: "lease_duration must be greater than renew_deadline",
		},
		{
			name: "renew deadline less than jittered retry period",
			modifyConfig: func(cfg *Config) {
				cfg.RenewDuration = 2200 * time.Millisecond
			},
			expectedErr: "renew_deadline must be greater than retry_period * JitterFactor",
		},
		{
			name: "renew deadline equals jittered retry period",
			modifyConfig: func(cfg *Config) {
				cfg.RenewDuration = 2400 * time.Millisecond
			},
			expectedErr: "renew_deadline must be greater than retry_period * JitterFactor",
		},
		{
			name: "renew deadline just above jittered retry period",
			modifyConfig: func(cfg *Config) {
				cfg.RenewDuration = 2400*time.Millisecond + time.Nanosecond
			},
		},
		{
			name: "valid nanosecond durations",
			modifyConfig: func(cfg *Config) {
				cfg.LeaseDuration = 3 * time.Nanosecond
				cfg.RenewDuration = 2 * time.Nanosecond
				cfg.RetryPeriod = time.Nanosecond
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.LeaseName = "test-lease"
			cfg.LeaseNamespace = "test-namespace"
			if tt.modifyConfig != nil {
				tt.modifyConfig(cfg)
			}

			err := cfg.Validate()
			if tt.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}
