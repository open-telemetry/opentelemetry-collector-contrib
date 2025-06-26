// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	ocfg, ok := factory.CreateDefaultConfig().(*Config)
	assert.True(t, ok)
	assert.Equal(t, ocfg.BackOffConfig, configretry.NewDefaultBackOffConfig())
	assert.Equal(t, ocfg.QueueSettings, exporterhelper.NewDefaultQueueConfig())
	assert.Equal(t, ocfg.TimeoutSettings, exporterhelper.NewDefaultTimeoutConfig())
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Metrics.Endpoint = testutil.GetAvailableLocalAddress(t)

	set := exportertest.NewNopSettings(metadata.Type)
	oexp, err := factory.CreateMetrics(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, oexp)
	require.NoError(t, oexp.Shutdown(context.Background()))
}

func TestCreateMetricsWithDomain(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Domain = "localhost"

	set := exportertest.NewNopSettings(metadata.Type)
	oexp, err := factory.CreateMetrics(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, oexp)
}

func TestCreateLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Logs.Endpoint = testutil.GetAvailableLocalAddress(t)

	set := exportertest.NewNopSettings(metadata.Type)
	oexp, err := factory.CreateLogs(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, oexp)
	require.NoError(t, oexp.Shutdown(context.Background()))
}

func TestCreateLogsWithDomain(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Domain = "localhost"
	set := exportertest.NewNopSettings(metadata.Type)
	oexp, err := factory.CreateLogs(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, oexp)
}

func TestCreateTraces(t *testing.T) {
	endpoint := testutil.GetAvailableLocalAddress(t)
	tests := []struct {
		name             string
		config           *Config
		mustFailOnCreate bool
		mustFailOnStart  bool
	}{
		{
			name: "UseSecure",
			config: &Config{
				Traces: configgrpc.ClientConfig{
					Endpoint: endpoint,
					TLS: configtls.ClientConfig{
						Insecure: false,
					},
				},
			},
		},
		{
			name: "Keepalive",
			config: &Config{
				Traces: configgrpc.ClientConfig{
					Endpoint: endpoint,
					Keepalive: &configgrpc.KeepaliveClientConfig{
						Time:                30 * time.Second,
						Timeout:             25 * time.Second,
						PermitWithoutStream: true,
					},
				},
			},
		},
		{
			name: "NoneCompression",
			config: &Config{
				Traces: configgrpc.ClientConfig{
					Endpoint:    endpoint,
					Compression: "none",
				},
			},
		},
		{
			name: "GzipCompression",
			config: &Config{
				Traces: configgrpc.ClientConfig{
					Endpoint:    endpoint,
					Compression: configcompression.TypeGzip,
				},
			},
		},
		{
			name: "SnappyCompression",
			config: &Config{
				Traces: configgrpc.ClientConfig{
					Endpoint:    endpoint,
					Compression: configcompression.TypeSnappy,
				},
			},
		},
		{
			name: "ZstdCompression",
			config: &Config{
				Traces: configgrpc.ClientConfig{
					Endpoint:    endpoint,
					Compression: configcompression.TypeZstd,
				},
			},
		},
		{
			name: "Headers",
			config: &Config{
				Traces: configgrpc.ClientConfig{
					Endpoint: endpoint,
					Headers: map[string]configopaque.String{
						"hdr1": "val1",
						"hdr2": "val2",
					},
				},
			},
		},
		{
			name: "NumConsumers",
			config: &Config{
				Traces: configgrpc.ClientConfig{
					Endpoint: endpoint,
				},
			},
		},
		{
			name: "CertPemFileError",
			config: &Config{
				Traces: configgrpc.ClientConfig{
					Endpoint: endpoint,
					TLS: configtls.ClientConfig{
						Config: configtls.Config{
							CAFile: "nosuchfile",
						},
					},
				},
			},
			mustFailOnStart: true,
		},
		{
			name: "UseDomain",
			config: &Config{
				Domain: "localhost",
				DomainSettings: configgrpc.ClientConfig{
					TLS: configtls.ClientConfig{
						Insecure: false,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			set := exportertest.NewNopSettings(metadata.Type)
			consumer, err := factory.CreateTraces(context.Background(), set, tt.config)
			if tt.mustFailOnCreate {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, consumer)
			err = consumer.Start(context.Background(), componenttest.NewNopHost())
			if tt.mustFailOnStart {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			err = consumer.Shutdown(context.Background())
			if err != nil {
				// Since the endpoint of OTLP exporter doesn't actually exist,
				// exporter may already stop because it cannot connect.
				assert.Equal(t, "rpc error: code = Canceled desc = grpc: the client connection is closing", err.Error())
			}
		})
	}
}

func TestCreateLogsWithDomainAndEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Domain = "	bad domain"

	cfg.Logs.Endpoint = testutil.GetAvailableLocalAddress(t)

	set := exportertest.NewNopSettings(metadata.Type)
	consumer, err := factory.CreateLogs(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, consumer)

	err = consumer.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	err = consumer.Shutdown(context.Background())
	if err != nil {
		// Since the endpoint of OTLP exporter doesn't actually exist,
		// exporter may already stop because it cannot connect.
		assert.Equal(t, "rpc error: code = Canceled desc = grpc: the client connection is closing", err.Error())
	}
}
