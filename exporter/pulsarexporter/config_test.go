// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 20 * time.Second,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
				Endpoint:                "pulsar://localhost:6650",
				Topic:                   "spans",
				Encoding:                "otlp-spans",
				TLSTrustCertsFilePath:   "ca.pem",
				Authentication:          Authentication{TLS: &TLS{CertFile: "cert.pem", KeyFile: "key.pem"}},
				MaxConnectionsPerBroker: 1,
				ConnectionTimeout:       5 * time.Second,
				OperationTimeout:        30 * time.Second,
				Producer: Producer{
					MaxReconnectToBroker:            nil,
					HashingScheme:                   "java_string_hash",
					CompressionLevel:                "default",
					CompressionType:                 "zstd",
					MaxPendingMessages:              100,
					BatcherBuilderType:              "key_based",
					PartitionsAutoDiscoveryInterval: 60000000000,
					BatchingMaxPublishDelay:         10000000,
					BatchingMaxMessages:             1000,
					BatchingMaxSize:                 128000,
					DisableBlockIfQueueFull:         false,
					DisableBatching:                 false,
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

func TestClientOptions(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	options := cfg.(*Config).clientOptions()

	assert.Equal(t, &pulsar.ClientOptions{
		URL:                     "pulsar://localhost:6650",
		TLSTrustCertsFilePath:   "ca.pem",
		Authentication:          pulsar.NewAuthenticationTLS("cert.pem", "key.pem"),
		ConnectionTimeout:       5 * time.Second,
		OperationTimeout:        30 * time.Second,
		MaxConnectionsPerBroker: 1,
	}, &options)

}
