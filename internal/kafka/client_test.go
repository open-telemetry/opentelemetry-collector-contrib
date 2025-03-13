// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
)

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
		"auth": {
			input: func() configkafka.ClientConfig {
				cfg := configkafka.NewDefaultClientConfig()
				cfg.Authentication.TLS = &configtls.ClientConfig{
					Config: configtls.Config{CAFile: "/nonexistent"},
				}
				return cfg
			}(),
			expectedErr: "failed to load TLS config",
		},
	} {
		t.Run(name, func(t *testing.T) {
			output, err := NewSaramaClientConfig(context.Background(), tt.input)
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
