// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configkafka // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
)

func TestClientConfig(t *testing.T) {
	testConfig(t, "client_config.yaml", NewDefaultClientConfig, map[string]struct {
		expected    ClientConfig
		expectedErr string
	}{
		"": {
			expected: NewDefaultClientConfig(),
		},
		"full": {
			expected: ClientConfig{
				Brokers:                              []string{"foo:123", "bar:456"},
				ResolveCanonicalBootstrapServersOnly: true,
				ClientID:                             "vip",
				ProtocolVersion:                      "1.2.3",
				TLS: &configtls.ClientConfig{
					Config: configtls.Config{
						CAFile:   "ca.pem",
						CertFile: "cert.pem",
						KeyFile:  "key.pem",
					},
				},
				Authentication: AuthenticationConfig{
					SASL: &SASLConfig{
						Mechanism: "PLAIN",
						Username:  "abc",
						Password:  "def",
					},
				},
				Metadata: MetadataConfig{
					Full:            false,
					RefreshInterval: 10 * time.Minute,
					Retry: MetadataRetryConfig{
						Max:     10,
						Backoff: 5 * time.Second,
					},
				},
				RackID:          "rack1",
				UseLeaderEpoch:  true,
				ConnIdleTimeout: 5 * time.Minute,
			},
		},
		"sasl_aws_msk_iam_oauthbearer": {
			expected: func() ClientConfig {
				cfg := NewDefaultClientConfig()
				cfg.Authentication.SASL = &SASLConfig{
					Mechanism: "AWS_MSK_IAM_OAUTHBEARER",
				}
				return cfg
			}(),
		},
		"sasl_aws_msk_iam_oauthbearer_with_region": {
			expected: func() ClientConfig {
				cfg := NewDefaultClientConfig()
				cfg.Authentication.SASL = &SASLConfig{
					Mechanism: "AWS_MSK_IAM_OAUTHBEARER",
					AWSMSK: AWSMSKConfig{
						Region: "us-east-1",
					},
				}
				return cfg
			}(),
		},
		"sasl_plain": {
			expected: func() ClientConfig {
				cfg := NewDefaultClientConfig()
				cfg.Authentication.SASL = &SASLConfig{
					Mechanism: "PLAIN",
					Username:  "abc",
					Password:  "def",
					Version:   1,
				}
				return cfg
			}(),
		},
		"legacy_auth_tls": {
			expected: func() ClientConfig {
				cfg := NewDefaultClientConfig()
				cfg.Authentication.TLS = &configtls.ClientConfig{
					Config: configtls.Config{
						CAFile:   "ca.pem",
						CertFile: "cert.pem",
						KeyFile:  "key.pem",
					},
				}
				return cfg
			}(),
		},
		"legacy_auth_plain_text": {
			expected: func() ClientConfig {
				cfg := NewDefaultClientConfig()
				cfg.Authentication.PlainText = &PlainTextConfig{
					Username: "abc",
					Password: "def",
				}
				return cfg
			}(),
		},
		"not_use_leader_epoch": {
			expected: func() ClientConfig {
				cfg := NewDefaultClientConfig()
				cfg.UseLeaderEpoch = false
				return cfg
			}(),
		},
		"conn_idle_timeout": {
			expected: func() ClientConfig {
				cfg := NewDefaultClientConfig()
				cfg.ConnIdleTimeout = 5 * time.Minute
				return cfg
			}(),
		},

		// Invalid configurations
		"brokers_required": {
			expectedErr: "brokers must be specified",
		},
		"invalid_protocol_version": {
			expectedErr: "invalid protocol version: invalid version `none`",
		},
		"sasl_invalid_mechanism": {
			expectedErr: "auth::sasl: mechanism should be one of 'PLAIN', 'AWS_MSK_IAM_OAUTHBEARER', 'SCRAM-SHA-256' or 'SCRAM-SHA-512'. configured value FANCY",
		},
		"sasl_invalid_version": {
			expectedErr: "auth::sasl: version has to be either 0 or 1. configured value -1",
		},
		"sasl_plain_username_required": {
			expectedErr: "auth::sasl: username is required",
		},
		"sasl_plain_password_required": {
			expectedErr: "auth::sasl: password is required",
		},
	})
}

func TestConsumerConfig(t *testing.T) {
	testConfig(t, "consumer_config.yaml", NewDefaultConsumerConfig, map[string]struct {
		expected    ConsumerConfig
		expectedErr string
	}{
		"": {
			expected: NewDefaultConsumerConfig(),
		},
		"full": {
			expected: ConsumerConfig{
				SessionTimeout:         5 * time.Second,
				HeartbeatInterval:      2 * time.Second,
				GroupID:                "throng",
				GroupRebalanceStrategy: "cooperative-sticky",
				InitialOffset:          "earliest",
				AutoCommit: AutoCommitConfig{
					Enable:   false,
					Interval: 10 * time.Minute,
				},
				MinFetchSize:          10,
				MaxFetchSize:          4096,
				MaxFetchWait:          1 * time.Second,
				MaxPartitionFetchSize: 4096,
			},
		},
		"zero_min_fetch_size": {
			expected: ConsumerConfig{
				SessionTimeout:    10 * time.Second,
				HeartbeatInterval: 3 * time.Second,
				GroupID:           "otel-collector",
				InitialOffset:     "latest",
				AutoCommit: AutoCommitConfig{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				MinFetchSize:          0,
				MaxFetchSize:          1048576,
				MaxFetchWait:          250 * time.Millisecond,
				MaxPartitionFetchSize: 1048576,
			},
		},

		// Invalid configurations
		"invalid_initial_offset": {
			expectedErr: "initial_offset should be one of 'latest' or 'earliest'. configured value middle",
		},
		"invalid_fetch_size": {
			expectedErr: "max_fetch_size (100) cannot be less than min_fetch_size (1000)",
		},
		"negative_min_fetch_size": {
			expectedErr: "min_fetch_size (-100) must be non-negative",
		},
	})
}

func TestProducerConfig(t *testing.T) {
	testConfig(t, "producer_config.yaml", NewDefaultProducerConfig, map[string]struct {
		expected    ProducerConfig
		expectedErr string
	}{
		"": {
			expected: NewDefaultProducerConfig(),
		},
		"full": {
			expected: func() ProducerConfig {
				cfg := NewDefaultProducerConfig()
				cfg.MaxMessageBytes = 1
				cfg.RequiredAcks = 0
				cfg.Compression = "gzip"
				cfg.CompressionParams.Level = 1
				cfg.FlushMaxMessages = 2
				return cfg
			}(),
		},
		"default_compression_level": {
			expected: func() ProducerConfig {
				cfg := NewDefaultProducerConfig()
				cfg.MaxMessageBytes = 1
				cfg.RequiredAcks = 0
				cfg.Compression = "zstd"
				// zero is treated as the codec-specific default
				cfg.CompressionParams.Level = 0
				cfg.FlushMaxMessages = 2
				return cfg
			}(),
		},
		"snappy_compression": {
			expected: func() ProducerConfig {
				cfg := NewDefaultProducerConfig()
				cfg.Compression = "snappy"
				return cfg
			}(),
		},
		"disable_auto_topic_creation": {
			expected: func() ProducerConfig {
				cfg := NewDefaultProducerConfig()
				cfg.AllowAutoTopicCreation = false
				return cfg
			}(),
		},
		"producer_linger": {
			expected: func() ProducerConfig {
				cfg := NewDefaultProducerConfig()
				cfg.Linger = 100 * time.Millisecond
				return cfg
			}(),
		},
		"producer_linger_1s": {
			expected: func() ProducerConfig {
				cfg := NewDefaultProducerConfig()
				cfg.Linger = 1 * time.Second
				return cfg
			}(),
		},
		"invalid_compression_level": {
			expectedErr: `unsupported parameters {Level:-123} for compression type "gzip"`,
		},
		"required_acks_all": {
			expected: func() ProducerConfig {
				cfg := NewDefaultProducerConfig()
				cfg.RequiredAcks = WaitForAll
				return cfg
			}(),
		},
		"custom_flush_max_messages": {
			expected: func() ProducerConfig {
				cfg := NewDefaultProducerConfig()
				cfg.FlushMaxMessages = 5000
				return cfg
			}(),
		},

		// Invalid configurations
		"invalid_compression": {
			expectedErr: `compression should be one of 'none', 'gzip', 'snappy', 'lz4', or 'zstd'. configured value is "brotli"`,
		},
		"invalid_required_acks": {
			expectedErr: "required_acks: expected 'all' (-1), 0, or 1; configured value is 3",
		},
		"flush_max_messages_zero": {
			expectedErr: "flush_max_messages (0) must be at least 1",
		},
		"flush_max_messages_negative": {
			expectedErr: "flush_max_messages (-1) must be at least 1",
		},
		"max_message_bytes_negative": {
			expectedErr: "max_message_bytes (-1000) must be non-negative",
		},
	})
}

func testConfig[ConfigStruct any](t *testing.T, filename string, defaultConfig func() ConfigStruct, testcases map[string]struct {
	expected    ConfigStruct
	expectedErr string
},
) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", filename))
	require.NoError(t, err)

	for name, tt := range testcases {
		t.Run(name, func(t *testing.T) {
			cfg := defaultConfig()

			sub, err := cm.Sub(component.NewIDWithName(component.MustNewType("kafka"), name).String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(&cfg))

			err = xconfmap.Validate(cfg)
			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, cfg)
			}
		})
	}
}
