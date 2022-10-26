// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          config.ComponentID
		expected    config.Receiver
		expectedErr error
	}{
		{
			id: config.NewComponentIDWithName(componentType, "primary"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(componentType)),
				Broker:           []string{"myHost:5671"},
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
			id:          config.NewComponentIDWithName(componentType, "noauth"),
			expectedErr: errMissingAuthDetails,
		},
		{
			id:          config.NewComponentIDWithName(componentType, "noqueue"),
			expectedErr: errMissingQueueName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalReceiver(sub, cfg))

			if tt.expectedErr != nil {
				assert.ErrorIs(t, cfg.Validate(), tt.expectedErr)
				return
			}
			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidateMissingAuth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "someQueue"
	err := cfg.Validate()
	assert.Equal(t, errMissingAuthDetails, err)
}

func TestConfigValidateMissingQueue(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.PlainText = &SaslPlainTextConfig{"Username", "Password"}
	err := cfg.Validate()
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
			err := cfg.Validate()
			assert.NoError(t, err)
		})
	}
}
