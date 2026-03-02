// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension

import (
	"net"
	"net/http"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	expected := &Config{
		Config: healthcheck.Config{
			LegacyConfig: healthcheck.HTTPLegacyConfig{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: "tcp",
						Endpoint:  "localhost:13133",
					},
				},
				Path: "/",
				CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
					Enabled:                  false,
					Interval:                 "5m",
					ExporterFailureThreshold: 5,
				},
			},
		},
	}
	assert.Equal(t, expected, cfg)

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_CreateLegacyExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = testutil.GetAvailableLocalAddress(t)

	ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)
	require.IsType(t, &healthCheckExtension{}, ext)
}

func TestLegacyExtensionLifecycle(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = testutil.GetAvailableLocalAddress(t)

	hcExt, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)

	require.NoError(t, hcExt.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, hcExt.Shutdown(t.Context())) })

	// Give a chance for the server goroutine to run.
	runtime.Gosched()

	client := &http.Client{}
	url := "http://" + cfg.NetAddr.Endpoint + cfg.Path

	resp, err := client.Get(url)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NoError(t, resp.Body.Close())
}

func TestLegacyExtensionPortAlreadyInUse(t *testing.T) {
	endpoint := testutil.GetAvailableLocalAddress(t)
	ln, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer ln.Close()

	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = endpoint

	hcExt, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)

	require.Error(t, hcExt.Start(t.Context(), componenttest.NewNopHost()))
}

func TestFactory_CreateV2Extension(t *testing.T) {
	prev := useComponentStatusGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(useComponentStatusGate.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(useComponentStatusGate.ID(), prev))
	})

	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = testutil.GetAvailableLocalAddress(t)

	ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)
	require.IsType(t, &healthcheck.HealthCheckExtension{}, ext)
}
