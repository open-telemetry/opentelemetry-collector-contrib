// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		option   func(conf *Config)
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			option: func(_ *Config) {
				// intentionally left blank so we use default config
			},
			expected: &Config{
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 10 * time.Second,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueConfig{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
				Topic:                                "spans",
				Encoding:                             "otlp_proto",
				PartitionTracesByID:                  true,
				PartitionMetricsByResourceAttributes: true,
				PartitionLogsByResourceAttributes:    true,
				Brokers:                              []string{"foo:123", "bar:456"},
				ClientID:                             "test_client_id",
				Authentication: configkafka.AuthenticationConfig{
					PlainText: &configkafka.PlainTextConfig{
						Username: "jdoe",
						Password: "pass",
					},
				},
				Metadata: Metadata{
					Full: false,
					Retry: MetadataRetry{
						Max:     15,
						Backoff: defaultMetadataRetryBackoff,
					},
				},
				Producer: Producer{
					MaxMessageBytes: 10000000,
					RequiredAcks:    sarama.WaitForAll,
					Compression:     "none",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, ""),
			option: func(conf *Config) {
				conf.Authentication = configkafka.AuthenticationConfig{
					SASL: &configkafka.SASLConfig{
						Username:  "jdoe",
						Password:  "pass",
						Mechanism: "PLAIN",
						Version:   0,
					},
				}
			},
			expected: &Config{
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 10 * time.Second,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueConfig{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
				Topic:                                "spans",
				Encoding:                             "otlp_proto",
				PartitionTracesByID:                  true,
				PartitionMetricsByResourceAttributes: true,
				PartitionLogsByResourceAttributes:    true,
				Brokers:                              []string{"foo:123", "bar:456"},
				ClientID:                             "test_client_id",
				Authentication: configkafka.AuthenticationConfig{
					PlainText: &configkafka.PlainTextConfig{
						Username: "jdoe",
						Password: "pass",
					},
					SASL: &configkafka.SASLConfig{
						Username:  "jdoe",
						Password:  "pass",
						Mechanism: "PLAIN",
						Version:   0,
					},
				},
				Metadata: Metadata{
					Full: false,
					Retry: MetadataRetry{
						Max:     15,
						Backoff: defaultMetadataRetryBackoff,
					},
				},
				Producer: Producer{
					MaxMessageBytes: 10000000,
					RequiredAcks:    sarama.WaitForAll,
					Compression:     "none",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, ""),
			option: func(conf *Config) {
				conf.ResolveCanonicalBootstrapServersOnly = true
			},
			expected: &Config{
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 10 * time.Second,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueConfig{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
				Topic:                                "spans",
				Encoding:                             "otlp_proto",
				PartitionTracesByID:                  true,
				PartitionMetricsByResourceAttributes: true,
				PartitionLogsByResourceAttributes:    true,
				Brokers:                              []string{"foo:123", "bar:456"},
				ClientID:                             "test_client_id",
				ResolveCanonicalBootstrapServersOnly: true,
				Authentication: configkafka.AuthenticationConfig{
					PlainText: &configkafka.PlainTextConfig{
						Username: "jdoe",
						Password: "pass",
					},
				},
				Metadata: Metadata{
					Full: false,
					Retry: MetadataRetry{
						Max:     15,
						Backoff: defaultMetadataRetryBackoff,
					},
				},
				Producer: Producer{
					MaxMessageBytes: 10000000,
					RequiredAcks:    sarama.WaitForAll,
					Compression:     "none",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := applyConfigOption(tt.option)

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidate_err_compression(t *testing.T) {
	config := &Config{
		Producer: Producer{
			Compression: "idk",
		},
	}

	err := config.Validate()
	assert.EqualError(t, err, "producer.compression should be one of 'none', 'gzip', 'snappy', 'lz4', or 'zstd'. configured value idk")
}

func Test_saramaProducerCompressionCodec(t *testing.T) {
	tests := map[string]struct {
		compression         string
		expectedCompression sarama.CompressionCodec
		expectedError       error
	}{
		"none": {
			compression:         "none",
			expectedCompression: sarama.CompressionNone,
			expectedError:       nil,
		},
		"gzip": {
			compression:         "gzip",
			expectedCompression: sarama.CompressionGZIP,
			expectedError:       nil,
		},
		"snappy": {
			compression:         "snappy",
			expectedCompression: sarama.CompressionSnappy,
			expectedError:       nil,
		},
		"lz4": {
			compression:         "lz4",
			expectedCompression: sarama.CompressionLZ4,
			expectedError:       nil,
		},
		"zstd": {
			compression:         "zstd",
			expectedCompression: sarama.CompressionZSTD,
			expectedError:       nil,
		},
		"unknown": {
			compression:         "unknown",
			expectedCompression: sarama.CompressionNone,
			expectedError:       fmt.Errorf("producer.compression should be one of 'none', 'gzip', 'snappy', 'lz4', or 'zstd'. configured value unknown"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			c, err := saramaProducerCompressionCodec(test.compression)
			assert.Equal(t, test.expectedCompression, c)
			assert.Equal(t, test.expectedError, err)
		})
	}
}
