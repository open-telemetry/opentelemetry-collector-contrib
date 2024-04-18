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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/grpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/http"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/localhostgate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{
		LegacyConfig: http.LegacyConfig{
			ServerConfig: confighttp.ServerConfig{
				Endpoint: localhostgate.EndpointForPort(defaultHTTPPort),
			},
			Path: "/",
		},
		HTTPConfig: &http.Config{
			ServerConfig: confighttp.ServerConfig{
				Endpoint: localhostgate.EndpointForPort(defaultHTTPPort),
			},
			Status: http.PathConfig{
				Enabled: true,
				Path:    "/status",
			},
			Config: http.PathConfig{
				Enabled: false,
				Path:    "/config",
			},
		},
		GRPCConfig: &grpc.Config{
			ServerConfig: configgrpc.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Endpoint:  localhostgate.EndpointForPort(defaultGRPCPort),
					Transport: "tcp",
				},
			},
		},
	}, cfg)

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ext, err := createExtension(ctx, extensiontest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestCreateExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = testutil.GetAvailableLocalAddress(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ext, err := createExtension(ctx, extensiontest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}
