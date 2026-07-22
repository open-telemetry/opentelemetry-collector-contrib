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
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	legacyServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	legacyServerConfig.WriteTimeout = 0
	legacyServerConfig.ReadHeaderTimeout = 0
	legacyServerConfig.IdleTimeout = 0
	legacyServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
	}
	legacyServerConfig.KeepAlivesEnabled = true
	httpServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	httpServerConfig.WriteTimeout = 0
	httpServerConfig.ReadHeaderTimeout = 0
	httpServerConfig.IdleTimeout = 0
	httpServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
	}
	httpServerConfig.KeepAlivesEnabled = true
	assert.Equal(t, &Config{
		LegacyConfig: healthcheck.HTTPLegacyConfig{
			ServerConfig: legacyServerConfig,
			Path:         "/",
		},
		HTTPConfig: &healthcheck.HTTPConfig{
			ServerConfig: httpServerConfig,
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
				Keepalive: configoptional.Some(configgrpc.NewDefaultKeepaliveServerConfig()),
			},
		},
	}, cfg)

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ext, err := createExtension(ctx, extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.NoError(t, ext.Shutdown(ctx))
}

func TestCreate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServerConfig.NetAddr.Endpoint = testutil.GetAvailableLocalAddress(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ext, err := createExtension(ctx, extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.NoError(t, ext.Shutdown(ctx))
}
