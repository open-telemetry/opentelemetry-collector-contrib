// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/rcrowley/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
)

func init() {
	// Disable the go-metrics registry, as there's a goroutine leak in the Sarama
	// code that uses it. See this stale issue: https://github.com/IBM/sarama/issues/1321
	//
	// Sarama docs suggest setting UseNilMetrics to true to disable metrics if they
	// are not needed, which is the case here. We only disable in tests to avoid
	// affecting other components that rely on go-metrics.
	metrics.UseNilMetrics = true
}

func TestNewSaramaClientConfig(t *testing.T) {
	for name, tt := range map[string]struct {
		input       configkafka.ClientConfig
		check       func(*testing.T, *sarama.Config)
		expectedErr string
	}{
		"default": {
			input: configkafka.NewDefaultClientConfig(),
			check: func(t *testing.T, cfg *sarama.Config) {
				expected := sarama.NewConfig()

				// Ignore uncomparable fields, which happen to be irrelevant
				// for the base client anyway.
				expected.Consumer.Group.Rebalance.GroupStrategies = nil
				expected.MetricRegistry = nil
				expected.Producer.Partitioner = nil
				cfg.Consumer.Group.Rebalance.GroupStrategies = nil
				cfg.MetricRegistry = nil
				cfg.Producer.Partitioner = nil

				// Our metadata defaults differ from those of Sarama's.
				defaultMetadataConfig := configkafka.NewDefaultMetadataConfig()
				expected.Metadata.Full = defaultMetadataConfig.Full
				expected.Metadata.Retry.Max = defaultMetadataConfig.Retry.Max
				expected.Metadata.Retry.Backoff = defaultMetadataConfig.Retry.Backoff

				assert.Equal(t, expected, cfg)
			},
		},
		"metadata": {
			input: func() configkafka.ClientConfig {
				cfg := configkafka.NewDefaultClientConfig()
				cfg.Metadata = configkafka.MetadataConfig{
					Full: false,
					Retry: configkafka.MetadataRetryConfig{
						Max:     123,
						Backoff: time.Minute,
					},
				}
				return cfg
			}(),
			check: func(t *testing.T, cfg *sarama.Config) {
				assert.False(t, cfg.Metadata.Full)
				assert.Equal(t, 123, cfg.Metadata.Retry.Max)
				assert.Equal(t, time.Minute, cfg.Metadata.Retry.Backoff)
				assert.Nil(t, cfg.Metadata.Retry.BackoffFunc)
			},
		},
		"resolve_canonical_bootstrap_servers_only": {
			input: func() configkafka.ClientConfig {
				cfg := configkafka.NewDefaultClientConfig()
				cfg.ResolveCanonicalBootstrapServersOnly = true
				return cfg
			}(),
			check: func(t *testing.T, cfg *sarama.Config) {
				assert.True(t, cfg.Net.ResolveCanonicalBootstrapServers)
			},
		},
		"protocol_version": {
			input: func() configkafka.ClientConfig {
				cfg := configkafka.NewDefaultClientConfig()
				cfg.ProtocolVersion = "3.1.2"
				return cfg
			}(),
			check: func(t *testing.T, cfg *sarama.Config) {
				assert.Equal(t, sarama.V3_1_2_0, cfg.Version)
			},
		},
		"tls": {
			input: func() configkafka.ClientConfig {
				cfg := configkafka.NewDefaultClientConfig()
				cfg.TLS = &configtls.ClientConfig{
					Config: configtls.Config{CAFile: "/nonexistent"},
				}
				return cfg
			}(),
			expectedErr: "failed to load TLS config",
		},
		"auth_tls": {
			input: func() configkafka.ClientConfig {
				cfg := configkafka.NewDefaultClientConfig()
				cfg.Authentication.TLS = &configtls.ClientConfig{
					Config: configtls.Config{CAFile: "/nonexistent"},
				}
				return cfg
			}(),
			expectedErr: "failed to load TLS config",
		},
		"auth_tls_ignored": {
			input: func() configkafka.ClientConfig {
				cfg := configkafka.NewDefaultClientConfig()
				cfg.TLS = &configtls.ClientConfig{}
				cfg.Authentication.TLS = &configtls.ClientConfig{
					Insecure: true,
				}
				return cfg
			}(),
			check: func(t *testing.T, cfg *sarama.Config) {
				assert.True(t, cfg.Net.TLS.Enable)
			},
		},
		"auth": {
			input: func() configkafka.ClientConfig {
				cfg := configkafka.NewDefaultClientConfig()
				cfg.Authentication.SASL = &configkafka.SASLConfig{
					Mechanism: "PLAIN",
				}
				return cfg
			}(),
			check: func(t *testing.T, cfg *sarama.Config) {
				assert.Equal(t, sarama.SASLMechanism("PLAIN"), cfg.Net.SASL.Mechanism)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			output, err := newSaramaClientConfig(context.Background(), tt.input)
			if tt.expectedErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, output)
				tt.check(t, output)
			}
		})
	}
}

func TestNewSaramaClient(t *testing.T) {
	_, clientConfig := kafkatest.NewCluster(t)
	client, err := NewSaramaClient(context.Background(), clientConfig)
	require.NoError(t, err)
	assert.NoError(t, client.Close())
}

func TestNewSaramaClient_SASL(t *testing.T) {
	_, clientConfig := kafkatest.NewCluster(t,
		kfake.EnableSASL(),
		kfake.Superuser("PLAIN", "plain_user", "plain_password"),
		kfake.Superuser("SCRAM-SHA-256", "scramsha256_user", "scramsha256_password"),
		kfake.Superuser("SCRAM-SHA-512", "scramsha512_user", "scramsha512_password"),
	)

	tryConnect := func(mechanism, username, password string) error {
		clientConfig := clientConfig // copy
		clientConfig.Authentication.SASL = &configkafka.SASLConfig{
			Mechanism: mechanism,
			Username:  username,
			Password:  password,
			Version:   1, // kfake only supports version 1
		}
		client, err := NewSaramaClient(context.Background(), clientConfig)
		if err != nil {
			return err
		}
		return client.Close()
	}

	type testcase struct {
		mechanism string
		username  string
		password  string
		expecErr  bool
	}

	for name, tt := range map[string]testcase{
		"PLAIN": {
			mechanism: "PLAIN",
			username:  "plain_user",
			password:  "plain_password",
		},
		"SCRAM-SHA-256": {
			mechanism: "SCRAM-SHA-256",
			username:  "scramsha256_user",
			password:  "scramsha256_password",
		},
		"SCRAM-SHA-512": {
			mechanism: "SCRAM-SHA-512",
			username:  "scramsha512_user",
			password:  "scramsha512_password",
		},
		"invalid_PLAIN": {
			mechanism: "PLAIN",
			username:  "scramsha256_user",
			password:  "scramsha256_password",
			expecErr:  true,
		},
		"invalid_SCRAM-SHA-256": {
			mechanism: "SCRAM-SHA-256",
			username:  "scramsha512_user",
			password:  "scramsha512_password",
			expecErr:  true,
		},
		"invalid_SCRAM-SHA-512": {
			mechanism: "SCRAM-SHA-512",
			username:  "scramsha256_user",
			password:  "scramsha256_password",
			expecErr:  true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := tryConnect(tt.mechanism, tt.username, tt.password)
			if tt.expecErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewSaramaClient_TLS(t *testing.T) {
	// We create an httptest.Server just so we can get its TLS configuration.
	httpServer := httptest.NewTLSServer(http.NewServeMux())
	defer httpServer.Close()
	serverTLS := httpServer.TLS
	caCert := httpServer.Certificate() // self-signed

	_, clientConfig := kafkatest.NewCluster(t, kfake.TLS(serverTLS))
	tryConnect := func(cfg configtls.ClientConfig) error {
		clientConfig := clientConfig // copy
		clientConfig.TLS = &cfg
		client, err := NewSaramaClient(context.Background(), clientConfig)
		if err != nil {
			return err
		}
		return client.Close()
	}

	t.Run("tls_valid_ca", func(t *testing.T) {
		t.Parallel()
		tlsConfig := configtls.NewDefaultClientConfig()
		tlsConfig.CAPem = configopaque.String(
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw}),
		)
		assert.NoError(t, tryConnect(tlsConfig))
	})

	t.Run("tls_insecure_skip_verify", func(t *testing.T) {
		t.Parallel()
		tlsConfig := configtls.NewDefaultClientConfig()
		tlsConfig.InsecureSkipVerify = true
		require.NoError(t, tryConnect(tlsConfig))
	})

	t.Run("legacy_auth_tls", func(t *testing.T) {
		t.Parallel()

		tlsConfig := configtls.NewDefaultClientConfig()
		tlsConfig.InsecureSkipVerify = true
		clientConfig := clientConfig // copy
		clientConfig.Authentication.TLS = &tlsConfig

		client, err := NewSaramaClient(context.Background(), clientConfig)
		require.NoError(t, err)
		assert.NoError(t, client.Close())

		// The legacy auth TLS config should be ignored when the
		// top-level TLS config is specified.
		invalidTLSConfig := configtls.NewDefaultClientConfig()
		clientConfig.TLS = &invalidTLSConfig
		_, err = NewSaramaClient(context.Background(), clientConfig)
		assert.ErrorContains(t, err, "x509: certificate signed by unknown authority")
	})

	t.Run("tls_unknown_ca", func(t *testing.T) {
		t.Parallel()
		config := configtls.NewDefaultClientConfig()
		err := tryConnect(config)
		require.Error(t, err)
		assert.ErrorContains(t, err, "x509: certificate signed by unknown authority")
	})

	t.Run("plaintext", func(t *testing.T) {
		t.Parallel()
		// Should fail because the server expects TLS.
		require.Error(t, tryConnect(configtls.ClientConfig{}))
	})
}
