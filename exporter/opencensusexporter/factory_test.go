// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensusexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
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
			name: "NoEndpoint",
			config: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "",
				},
				NumWorkers: 3,
			},
			mustFailOnCreate: true,
		},
		{
			name: "ZeroNumWorkers",
			config: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: endpoint,
					TLS: configtls.ClientConfig{
						Insecure: false,
					},
				},
				NumWorkers: 0,
			},
			mustFailOnCreate: true,
		},
		{
			name: "UseSecure",
			config: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: endpoint,
					TLS: configtls.ClientConfig{
						Insecure: false,
					},
				},
				NumWorkers: 3,
			},
		},
		{
			name: "Keepalive",
			config: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: endpoint,
					Keepalive: &configgrpc.KeepaliveClientConfig{
						Time:                30 * time.Second,
						Timeout:             25 * time.Second,
						PermitWithoutStream: true,
					},
				},
				NumWorkers: 3,
			},
		},
		{
			name: "Compression",
			config: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint:    endpoint,
					Compression: "gzip",
				},
				NumWorkers: 3,
			},
		},
		{
			name: "Headers",
			config: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: endpoint,
					Headers: map[string]configopaque.String{
						"hdr1": "val1",
						"hdr2": "val2",
					},
				},
				NumWorkers: 3,
			},
		},
		{
			name: "CompressionError",
			config: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint:    endpoint,
					Compression: "unknown compression",
				},
				NumWorkers: 3,
			},
			mustFailOnCreate: false,
			mustFailOnStart:  true,
		},
		{
			name: "CaCert",
			config: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: endpoint,
					TLS: configtls.ClientConfig{
						Config: configtls.Config{
							CAFile: "testdata/test_cert.pem",
						},
					},
				},
				NumWorkers: 3,
			},
		},
		{
			name: "CertPemFileError",
			config: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: endpoint,
					TLS: configtls.ClientConfig{
						Config: configtls.Config{
							CAFile: "nosuchfile",
						},
					},
				},
				NumWorkers: 3,
			},
			mustFailOnCreate: false,
			mustFailOnStart:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := exportertest.NewNopSettings(metadata.Type)
			tExporter, tErr := createTracesExporter(context.Background(), set, tt.config)
			checkErrorsAndStartAndShutdown(t, tExporter, tErr, tt.mustFailOnCreate, tt.mustFailOnStart)
			mExporter, mErr := createMetricsExporter(context.Background(), set, tt.config)
			checkErrorsAndStartAndShutdown(t, mExporter, mErr, tt.mustFailOnCreate, tt.mustFailOnStart)
		})
	}
}

func checkErrorsAndStartAndShutdown(t *testing.T, exporter component.Component, err error, mustFailOnCreate, mustFailOnStart bool) {
	if mustFailOnCreate {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	assert.NotNil(t, exporter)

	sErr := exporter.Start(context.Background(), componenttest.NewNopHost())
	if mustFailOnStart {
		require.Error(t, sErr)
		return
	}
	require.NoError(t, sErr)
	require.NoError(t, exporter.Shutdown(context.Background()))
}
