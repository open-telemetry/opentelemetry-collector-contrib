// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/metadata"
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
			id: component.NewIDWithName(metadata.Type, "primary"),
			expected: &Config{
				Broker: []string{"myHost:5671"},
				Auth: Authentication{
					PlainText: &SaslPlainTextConfig{
						Username: "otel",
						Password: "otel01$",
					},
				},
				Queue:      "queue://#trace-profile123",
				MaxUnacked: 1234,
				TLS: configtls.TLSClientSetting{
					Insecure:           false,
					InsecureSkipVerify: false,
				},
				Flow: FlowControl{
					DelayedRetry: &FlowControlDelayedRetry{
						Delay: 1 * time.Second,
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "noauth"),
			expectedErr: errMissingAuthDetails,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "noqueue"),
			expectedErr: errMissingQueueName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expectedErr != nil {
				assert.ErrorIs(t, component.ValidateConfig(cfg), tt.expectedErr)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidateMissingAuth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "someQueue"
	err := component.ValidateConfig(cfg)
	assert.Equal(t, errMissingAuthDetails, err)
}

func TestConfigValidateMissingQueue(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.PlainText = &SaslPlainTextConfig{"Username", "Password"}
	err := component.ValidateConfig(cfg)
	assert.Equal(t, errMissingQueueName, err)
}

func TestConfigValidateMissingFlowControl(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "someQueue"
	cfg.Auth.PlainText = &SaslPlainTextConfig{"Username", "Password"}
	// this should never happen in reality, test validation anyway
	cfg.Flow.DelayedRetry = nil
	err := cfg.Validate()
	assert.Equal(t, errMissingFlowControl, err)
}

func TestConfigValidateInvalidFlowControlDelayedRetryDelay(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "someQueue"
	cfg.Auth.PlainText = &SaslPlainTextConfig{"Username", "Password"}
	cfg.Flow.DelayedRetry = &FlowControlDelayedRetry{
		Delay: -30 * time.Second,
	}
	err := cfg.Validate()
	assert.Equal(t, errInvalidDelayedRetryDelay, err)
}

func TestConfigValidateSuccess(t *testing.T) {
	successCases := map[string]func(*Config){
		"With Plaintext Auth": func(c *Config) {
			c.Auth.PlainText = &SaslPlainTextConfig{"Username", "Password"}
		},
		"With XAuth2 Auth": func(c *Config) {
			c.Auth.XAuth2 = &SaslXAuth2Config{
				Username: "Username",
				Bearer:   "Bearer",
			}
		},
		"With External Auth": func(c *Config) {
			c.Auth.External = &SaslExternalConfig{}
		},
	}

	for caseName, configure := range successCases {
		t.Run(caseName, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Queue = "someQueue"
			configure(cfg)
			err := component.ValidateConfig(cfg)
			assert.NoError(t, err)
		})
	}
}
