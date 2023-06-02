// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
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
					Timeout: 10 * time.Second,
				},
				RetrySettings: exporterhelper.RetrySettings{
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
				Topic:    "spans",
				Encoding: "otlp_proto",
				Brokers:  []string{"foo:123", "bar:456"},
				Authentication: Authentication{
					PlainText: &PlainTextConfig{
						Username: "jdoe",
						Password: "pass",
					},
					SASL: &SASLConfig{
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
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := applyConfigOption(func(conf *Config) {
				// config.Validate() reads the Authentication.SASL struct, but it's not present
				// in the default config. This sets it to avoid a segfault during testing.
				conf.Authentication = Authentication{
					SASL: &SASLConfig{
						Username:  "jdoe",
						Password:  "pass",
						Mechanism: "PLAIN",
						Version:   0,
					},
				}
			})

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
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
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "producer.compression should be one of 'none', 'gzip', 'snappy', 'lz4', or 'zstd'. configured value idk")
}

func TestValidate_sasl_username(t *testing.T) {
	config := &Config{
		Producer: Producer{
			Compression: "none",
		},
		Authentication: Authentication{
			SASL: &SASLConfig{
				Username:  "",
				Password:  "pass",
				Mechanism: "PLAIN",
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "auth.sasl.username is required")
}

func TestValidate_sasl_password(t *testing.T) {
	config := &Config{
		Producer: Producer{
			Compression: "none",
		},
		Authentication: Authentication{
			SASL: &SASLConfig{
				Username:  "jdoe",
				Password:  "",
				Mechanism: "PLAIN",
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "auth.sasl.password is required")
}

func TestValidate_sasl_mechanism(t *testing.T) {
	config := &Config{
		Producer: Producer{
			Compression: "none",
		},
		Authentication: Authentication{
			SASL: &SASLConfig{
				Username:  "jdoe",
				Password:  "pass",
				Mechanism: "FAKE",
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "auth.sasl.mechanism should be one of 'PLAIN', 'AWS_MSK_IAM', 'SCRAM-SHA-256' or 'SCRAM-SHA-512'. configured value FAKE")
}

func TestValidate_sasl_version(t *testing.T) {
	config := &Config{
		Producer: Producer{
			Compression: "none",
		},
		Authentication: Authentication{
			SASL: &SASLConfig{
				Username:  "jdoe",
				Password:  "pass",
				Mechanism: "PLAIN",
				Version:   42,
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "auth.sasl.version has to be either 0 or 1. configured value 42")
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
			assert.Equal(t, c, test.expectedCompression)
			assert.Equal(t, err, test.expectedError)
		})
	}
}
