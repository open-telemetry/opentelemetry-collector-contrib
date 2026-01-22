// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

func TestLegacyReadyNotReadyBehavior(t *testing.T) {
	transport := &http.Transport{
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: transport,
	}
	t.Cleanup(func() {
		transport.CloseIdleConnections()
	})

	f := NewFactory()
	port := testutil.GetAvailablePort(t)
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = fmt.Sprintf("localhost:%d", port)
	ext, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	require.IsType(t, &healthCheckExtension{}, ext)

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		// Use Background context for shutdown in cleanup to avoid cancellation issues.
		//nolint:usetesting // cleanup may run after the test context is cancelled
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, ext.Shutdown(ctx))
	})

	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/", port))
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.JSONEq(t, `{"status":"Server not available","upSince":"0001-01-01T00:00:00Z","uptime":""}`, string(body))

	pipelineWatcher, ok := ext.(extensioncapabilities.PipelineWatcher)
	require.True(t, ok)

	require.NoError(t, pipelineWatcher.Ready())

	assert.Eventually(t, func() bool {
		checkResp, checkErr := client.Get(fmt.Sprintf("http://localhost:%d/", port))
		if checkErr != nil {
			return false
		}
		defer checkResp.Body.Close()
		return checkResp.StatusCode == http.StatusOK
	}, 150*time.Millisecond, 5*time.Millisecond)

	resp, err = client.Get(fmt.Sprintf("http://localhost:%d/", port))
	require.NoError(t, err)
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Contains(t, buf.String(), `"status":"Server available"`)

	require.NoError(t, pipelineWatcher.NotReady())

	assert.Eventually(t, func() bool {
		checkResp, checkErr := client.Get(fmt.Sprintf("http://localhost:%d/", port))
		if checkErr != nil {
			return false
		}
		defer checkResp.Body.Close()
		return checkResp.StatusCode == http.StatusServiceUnavailable
	}, 150*time.Millisecond, 5*time.Millisecond)
}

func TestV2ExtensionEnabledByGate(t *testing.T) {
	prev := useComponentStatusGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(useComponentStatusGate.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(useComponentStatusGate.ID(), prev))
	})

	transport := &http.Transport{
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: transport,
	}
	t.Cleanup(func() {
		transport.CloseIdleConnections()
	})

	f := NewFactory()
	port := testutil.GetAvailablePort(t)
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = fmt.Sprintf("localhost:%d", port)
	ext, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	require.IsType(t, &healthcheck.HealthCheckExtension{}, ext)

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		// Use Background context for shutdown in cleanup to avoid cancellation issues.
		//nolint:usetesting // cleanup may run after the test context is cancelled
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, ext.Shutdown(ctx))
	})

	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/", port))
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}
