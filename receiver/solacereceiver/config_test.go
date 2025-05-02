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
	"go.opentelemetry.io/collector/confmap/xconfmap"

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
				TLS: configtls.ClientConfig{
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
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expectedErr != nil {
				assert.ErrorContains(t, xconfmap.Validate(cfg), tt.expectedErr.Error())
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidateMissingAuth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "someQueue"
	err := xconfmap.Validate(cfg)
	assert.ErrorContains(t, err, errMissingAuthDetails.Error())
}

func TestConfigValidateMultipleAuth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "someQueue"
	cfg.Auth.PlainText = &SaslPlainTextConfig{Username: "Username", Password: "Password"}
	cfg.Auth.XAuth2 = &SaslXAuth2Config{Username: "Username", Bearer: "Bearer"}
	err := xconfmap.Validate(cfg)
	assert.ErrorContains(t, err, errTooManyAuthDetails.Error())
}

func TestConfigValidateMissingQueue(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.PlainText = &SaslPlainTextConfig{Username: "Username", Password: "Password"}
	err := xconfmap.Validate(cfg)
	assert.ErrorContains(t, err, errMissingQueueName.Error())
}

func TestConfigValidateMissingFlowControl(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "someQueue"
	cfg.Auth.PlainText = &SaslPlainTextConfig{Username: "Username", Password: "Password"}
	// this should never happen in reality, test validation anyway
	cfg.Flow.DelayedRetry = nil
	err := cfg.Validate()
	assert.ErrorContains(t, err, errMissingFlowControl.Error())
}

func TestConfigValidateInvalidFlowControlDelayedRetryDelay(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "someQueue"
	cfg.Auth.PlainText = &SaslPlainTextConfig{Username: "Username", Password: "Password"}
	cfg.Flow.DelayedRetry = &FlowControlDelayedRetry{
		Delay: -30 * time.Second,
	}
	err := cfg.Validate()
	assert.ErrorContains(t, err, errInvalidDelayedRetryDelay.Error())
}

func TestConfigValidateSuccess(t *testing.T) {
	successCases := map[string]func(*Config){
		"With Plaintext Auth": func(c *Config) {
			c.Auth.PlainText = &SaslPlainTextConfig{Username: "Username", Password: "Password"}
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
			err := xconfmap.Validate(cfg)
			assert.NoError(t, err)
		})
	}
}
