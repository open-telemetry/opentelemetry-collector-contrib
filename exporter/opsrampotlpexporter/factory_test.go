// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package opsrampotlpexporter

import (
	"context"
	"path/filepath"
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
	assert.Equal(t, ocfg.QueueSettings, exporterhelper.NewDefaultQueueSettings())
	assert.Equal(t, ocfg.TimeoutSettings, exporterhelper.NewDefaultTimeoutSettings())
	assert.Equal(t, ocfg.Compression, configcompression.TypeGzip)
}

func TestCreateMetricsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = testutil.GetAvailableLocalAddress(t)
	cfg.Security = SecuritySettings{
		OAuthServiceURL: "https://asura.opsramp.net/auth/oauth/token?agent=true",
		ClientID:        "mamRxRJB796HYtWYxqeDzeEXCKSswnsr",
		ClientSecret:    "Da2achZqvHF7tKDaSP3FCkHE2PKcY6twRxwZEnEYQHc5GADgHy5VZDBxdeKhNbrw",
	}

	set := exportertest.NewNopSettings()
	oexp, err := factory.CreateMetricsExporter(context.Background(), set, cfg)
	require.Nil(t, err)
	require.NotNil(t, oexp)
}

func TestCreateTracesExporter(t *testing.T) {
	endpoint := testutil.GetAvailableLocalAddress(t)
	tests := []struct {
		name             string
		config           Config
		mustFailOnCreate bool
		mustFailOnStart  bool
	}{
		{
			name: "NoEndpoint",
			config: Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "",
				},
			},
			mustFailOnCreate: true,
		},
		{
			name: "UseSecure",
			config: Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: endpoint,
					TLSSetting: configtls.ClientConfig{
						Insecure: false,
					},
				},
			},
		},
		{
			name: "Keepalive",
			config: Config{
				ClientConfig: configgrpc.ClientConfig{
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
			config: Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint:    endpoint,
					Compression: "none",
				},
			},
		},
		{
			name: "GzipCompression",
			config: Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint:    endpoint,
					Compression: configcompression.TypeGzip,
				},
			},
		},
		{
			name: "SnappyCompression",
			config: Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint:    endpoint,
					Compression: configcompression.TypeSnappy,
				},
			},
		},
		{
			name: "ZstdCompression",
			config: Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint:    endpoint,
					Compression: configcompression.TypeZstd,
				},
			},
		},
		{
			name: "Headers",
			config: Config{
				ClientConfig: configgrpc.ClientConfig{
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
			config: Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: endpoint,
				},
			},
		},
		{
			name: "CaCert",
			config: Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: endpoint,
					TLSSetting: configtls.ClientConfig{
						Config: configtls.Config{
							CAFile: filepath.Join("testdata", "test_cert.pem"),
						},
					},
				},
			},
		},
		{
			name: "CertPemFileError",
			config: Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: endpoint,
					TLSSetting: configtls.ClientConfig{
						Config: configtls.Config{
							CAFile: "nosuchfile",
						},
					},
				},
			},
			mustFailOnStart: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			set := exportertest.NewNopSettings()
			consumer, err := factory.CreateTracesExporter(context.Background(), set, &tt.config)
			if tt.mustFailOnCreate {
				assert.NotNil(t, err)
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
				assert.Equal(t, err.Error(), "rpc error: code = Canceled desc = grpc: the client connection is closing")
			}
		})
	}
}

func TestCreateLogsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = testutil.GetAvailableLocalAddress(t)

	set := exportertest.NewNopSettings()
	oexp, err := factory.CreateLogsExporter(context.Background(), set, cfg)
	require.Nil(t, err)
	require.NotNil(t, oexp)
}
