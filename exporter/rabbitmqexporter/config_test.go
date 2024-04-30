// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "test-config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig().(*Config),
		},
		{
			id: component.NewIDWithName(metadata.Type, "all_fields"),
			expected: &Config{
				Connection: ConnectionConfig{
					Endpoint: "amqp://localhost:5672",
					VHost:    "vhost1",
					Auth: AuthConfig{
						SASL: SASLConfig{
							Username: "user",
							Password: "pass",
						},
					},
				},
				Routing: RoutingConfig{
					RoutingKey: "custom_routing_key",
				},
				MessageBodyEncoding: "otlp_json",
				Durable:             false,
				RetrySettings: configretry.BackOffConfig{
					Enabled: true,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "mandatory_fields"),
			expected: &Config{
				Connection: ConnectionConfig{
					Endpoint: "amqp://localhost:5672",
					VHost:    "",
					Auth: AuthConfig{
						SASL: SASLConfig{
							Username: "user",
							Password: "pass",
						},
					},
				},
				MessageBodyEncoding: "otlp_proto",
				Durable:             true,
				RetrySettings: configretry.BackOffConfig{
					Enabled: false,
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
