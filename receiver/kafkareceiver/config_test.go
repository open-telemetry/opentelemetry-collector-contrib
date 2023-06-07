// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
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
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				Topic:         "spans",
				Encoding:      "otlp_proto",
				Brokers:       []string{"foo:123", "bar:456"},
				ClientID:      "otel-collector",
				GroupID:       "otel-collector",
				InitialOffset: "latest",
				Authentication: kafkaexporter.Authentication{
					TLS: &configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile:   "ca.pem",
							CertFile: "cert.pem",
							KeyFile:  "key.pem",
						},
					},
				},
				Metadata: kafkaexporter.Metadata{
					Full: true,
					Retry: kafkaexporter.MetadataRetry{
						Max:     10,
						Backoff: time.Second * 5,
					},
				},
				AutoCommit: AutoCommit{
					Enable:   true,
					Interval: 1 * time.Second,
				},
			},
		},
		{

			id: component.NewIDWithName(metadata.Type, "logs"),
			expected: &Config{
				Topic:         "logs",
				Encoding:      "direct",
				Brokers:       []string{"coffee:123", "foobar:456"},
				ClientID:      "otel-collector",
				GroupID:       "otel-collector",
				InitialOffset: "earliest",
				Authentication: kafkaexporter.Authentication{
					TLS: &configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile:   "ca.pem",
							CertFile: "cert.pem",
							KeyFile:  "key.pem",
						},
					},
				},
				Metadata: kafkaexporter.Metadata{
					Full: true,
					Retry: kafkaexporter.MetadataRetry{
						Max:     10,
						Backoff: time.Second * 5,
					},
				},
				AutoCommit: AutoCommit{
					Enable:   true,
					Interval: 1 * time.Second,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "avroLogs"),
			expected: &Config{
				Topic:         "avroLogs",
				Encoding:      "avro",
				Brokers:       []string{"coffee:123", "foobar:456"},
				ClientID:      "otel-collector",
				GroupID:       "otel-collector",
				InitialOffset: "latest",
				Authentication: kafkaexporter.Authentication{
					TLS: &configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile:   "ca.pem",
							CertFile: "cert.pem",
							KeyFile:  "key.pem",
						},
					},
				},
				Metadata: kafkaexporter.Metadata{
					Full: true,
					Retry: kafkaexporter.MetadataRetry{
						Max:     10,
						Backoff: time.Second * 5,
					},
				},
				AutoCommit: AutoCommit{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				Avro: Avro{
					Schema: "{\"type\":\"record\",\"name\":\"test\",\"fields\":[]}\n",
					Mapping: map[string]string{
						"hostname":   "resource.attributes.hostname",
						"properties": "resource.attributes.properties",
						"message":    "body",
					},
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
