// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckv2extension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{
		LegacyConfig: healthcheck.HTTPLegacyConfig{
			ServerConfig: confighttp.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Transport: "tcp",
					Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
				},
			},
			Path: "/",
		},
		HTTPConfig: &healthcheck.HTTPConfig{
			ServerConfig: confighttp.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Transport: "tcp",
					Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
				},
			},
			Status: healthcheck.PathConfig{
				Enabled: true,
				Path:    "/status",
			},
			Config: healthcheck.PathConfig{
				Enabled: false,
				Path:    "/config",
			},
		},
		GRPCConfig: &healthcheck.GRPCConfig{
			ServerConfig: configgrpc.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Endpoint:  testutil.EndpointForPort(healthcheck.DefaultGRPCPort),
					Transport: "tcp",
				},
			},
		},
	}, cfg)

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ext, err := createExtension(ctx, extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestCreate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = testutil.GetAvailableLocalAddress(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ext, err := createExtension(ctx, extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}
