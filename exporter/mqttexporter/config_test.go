// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mqttexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Connection: ConnectionConfig{
					Endpoint: "tcp://localhost:1883",
					Auth: AuthConfig{
						Plain: PlainAuth{
							Username: "test",
							Password: "test",
						},
					},
				},
				Topic: TopicConfig{
					Topic: "test/topic",
				},
				QoS:    1,
				Retain: false,
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			config: &Config{
				Connection: ConnectionConfig{
					Auth: AuthConfig{
						Plain: PlainAuth{
							Username: "test",
							Password: "test",
						},
					},
				},
				Topic: TopicConfig{
					Topic: "test/topic",
				},
				QoS:    1,
				Retain: false,
			},
			wantErr: true,
		},
		{
			name: "missing username",
			config: &Config{
				Connection: ConnectionConfig{
					Endpoint: "tcp://localhost:1883",
					Auth: AuthConfig{
						Plain: PlainAuth{
							Password: "test",
						},
					},
				},
				Topic: TopicConfig{
					Topic: "test/topic",
				},
				QoS:    1,
				Retain: false,
			},
			wantErr: true,
		},
		{
			name: "valid config with TLS",
			config: &Config{
				Connection: ConnectionConfig{
					Endpoint: "tcp://localhost:1883",
					TLSConfig: &configtls.ClientConfig{
						Insecure: true,
					},
					Auth: AuthConfig{
						Plain: PlainAuth{
							Username: "test",
							Password: "test",
						},
					},
				},
				Topic: TopicConfig{
					Topic: "test/topic",
				},
				QoS:    1,
				Retain: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	config := createDefaultConfig().(*Config)

	// Test default values from createDefaultConfig
	assert.Equal(t, byte(1), config.QoS)
	assert.Equal(t, false, config.Retain)
	assert.Equal(t, time.Second*10, config.Connection.ConnectionTimeout)
	assert.Equal(t, time.Second*30, config.Connection.KeepAlive)
	assert.Equal(t, time.Second*5, config.Connection.PublishConfirmationTimeout)
}
