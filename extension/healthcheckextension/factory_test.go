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
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	expected := &healthcheck.Config{
		LegacyConfig: healthcheck.HTTPLegacyConfig{
			ServerConfig: confighttp.ServerConfig{
				Endpoint: "localhost:13133",
			},
			Path: "/",
			CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
				Enabled:                  false,
				Interval:                 "5m",
				ExporterFailureThreshold: 5,
			},
			UseV2: false,
		},
	}
	assert.Equal(t, expected, cfg)

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestFactory_Create(t *testing.T) {
	cfg := createDefaultConfig().(*healthcheck.Config)
	cfg.Endpoint = testutil.GetAvailableLocalAddress(t)

	ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestHealthCheckExtensionBasic(t *testing.T) {
	// Test that the extension can be created and started with the new shared implementation
	cfg := createDefaultConfig().(*healthcheck.Config)
	cfg.Endpoint = testutil.GetAvailableLocalAddress(t)

	hcExt, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)
	require.NotNil(t, hcExt)

	require.NoError(t, hcExt.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, hcExt.Shutdown(t.Context())) })

	// Give a chance for the server goroutine to run.
	runtime.Gosched()

	// Test basic endpoint availability
	client := &http.Client{}
	url := "http://" + cfg.Endpoint + cfg.Path

	resp, err := client.Get(url)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NoError(t, resp.Body.Close())
}

func TestHealthCheckExtensionPortAlreadyInUse(t *testing.T) {
	endpoint := testutil.GetAvailableLocalAddress(t)
	ln, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer ln.Close()

	cfg := createDefaultConfig().(*healthcheck.Config)
	cfg.Endpoint = endpoint

	hcExt, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	require.NoError(t, err)
	require.NotNil(t, hcExt)

	require.Error(t, hcExt.Start(t.Context(), componenttest.NewNopHost()))
}
