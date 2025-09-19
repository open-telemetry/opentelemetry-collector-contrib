// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension

import (
	"bytes"
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

func Test_SimpleHealthCheck(t *testing.T) {
	// This test verifies that the migration to the shared healthcheck implementation
	// maintains full backward compatibility with the original Ready/NotReady behavior.
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42256
	//
	// The extension now works exactly as before:
	// - Ready() calls make the health endpoint return 200 OK
	// - NotReady() calls make the health endpoint return 503 Service Unavailable
	// - This is a true drop-in replacement with no breaking changes

	// Create a custom HTTP client that closes connections to avoid goroutine leaks
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
	cfg := f.CreateDefaultConfig().(*healthcheck.Config)
	cfg.Endpoint = fmt.Sprintf("localhost:%d", port)
	e, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	err = e.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, e.Shutdown(t.Context()))
	})
	// Test initial state - should be unavailable
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/", port))
	require.NoError(t, err)
	assert.Equal(t, "503 Service Unavailable", resp.Status)
	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	assert.JSONEq(t, `{"status":"Server not available","upSince":"0001-01-01T00:00:00Z","uptime":""}`, buf.String())

	// Verify the extension implements PipelineWatcher interface as requested
	pipelineWatcher, ok := e.(extensioncapabilities.PipelineWatcher)
	require.True(t, ok, "Extension should implement PipelineWatcher interface")

	// Test Ready() method - should change health status to available
	err = pipelineWatcher.Ready()
	require.NoError(t, err, "Ready() should not return an error")

	// Wait for the async status change to propagate and health endpoint to become available
	assert.Eventually(t, func() bool {
		checkResp, checkErr := client.Get(fmt.Sprintf("http://localhost:%d/", port))
		if checkErr != nil {
			return false
		}
		defer checkResp.Body.Close()
		return checkResp.StatusCode == http.StatusOK
	}, 100*time.Millisecond, 5*time.Millisecond, "Health endpoint should become OK after Ready()")

	// Verify the response body contains the expected available message
	resp, err = client.Get(fmt.Sprintf("http://localhost:%d/", port))
	require.NoError(t, err)
	buf.Reset()
	_, err = io.Copy(&buf, resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Contains(t, buf.String(), `{"status":"Server available","upSince":"`)

	// Test NotReady() method - should change health status back to unavailable
	err = pipelineWatcher.NotReady()
	require.NoError(t, err, "NotReady() should not return an error")

	// Wait for the async status change to propagate and health endpoint to become unavailable
	assert.Eventually(t, func() bool {
		checkResp, checkErr := client.Get(fmt.Sprintf("http://localhost:%d/", port))
		if checkErr != nil {
			return false
		}
		defer checkResp.Body.Close()
		return checkResp.StatusCode == http.StatusServiceUnavailable
	}, 100*time.Millisecond, 5*time.Millisecond, "Health endpoint should become unavailable after NotReady()")

	// Verify the response body contains the expected unavailable message
	resp, err = client.Get(fmt.Sprintf("http://localhost:%d/", port))
	require.NoError(t, err)
	buf.Reset()
	_, err = io.Copy(&buf, resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	assert.JSONEq(t, `{"status":"Server not available","upSince":"0001-01-01T00:00:00Z","uptime":""}`, buf.String())
}

func Test_SimpleHealthCheck_WithoutCompatibilityWrapper(t *testing.T) {
	// This test verifies behavior when the compatibility wrapper is disabled via feature gate.
	// In this mode, Ready/NotReady calls don't directly affect health status - it's determined
	// by component status events like in the v2 implementation.

	// Create a custom HTTP client that closes connections to avoid goroutine leaks
	transport := &http.Transport{
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: transport,
	}
	t.Cleanup(func() {
		transport.CloseIdleConnections()
	})

	// Temporarily disable the compatibility wrapper feature gate for this test
	originalValue := useCompatibilityWrapperGate.IsEnabled()
	err := featuregate.GlobalRegistry().Set(useCompatibilityWrapperGate.ID(), false)
	require.NoError(t, err)
	t.Cleanup(func() {
		resetErr := featuregate.GlobalRegistry().Set(useCompatibilityWrapperGate.ID(), originalValue)
		require.NoError(t, resetErr)
	})

	f := NewFactory()
	port := testutil.GetAvailablePort(t)
	cfg := f.CreateDefaultConfig().(*healthcheck.Config)
	cfg.Endpoint = fmt.Sprintf("localhost:%d", port)
	e, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	err = e.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, e.Shutdown(t.Context()))
	})

	// Test initial state - should be unavailable
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/", port))
	require.NoError(t, err)
	assert.Equal(t, "503 Service Unavailable", resp.Status)
	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	assert.JSONEq(t, `{"status":"Server not available","upSince":"0001-01-01T00:00:00Z","uptime":""}`, buf.String())

	// Verify the extension implements PipelineWatcher interface
	pipelineWatcher, ok := e.(extensioncapabilities.PipelineWatcher)
	require.True(t, ok, "Extension should implement PipelineWatcher interface")

	// Test Ready() method - with wrapper disabled, this should NOT change health status
	err = pipelineWatcher.Ready()
	require.NoError(t, err, "Ready() should not return an error")

	// Health status should remain unavailable since no component status events were sent
	resp, err = client.Get(fmt.Sprintf("http://localhost:%d/", port))
	require.NoError(t, err)
	assert.Equal(t, "503 Service Unavailable", resp.Status, "Health endpoint should remain unavailable when wrapper is disabled")

	// Test NotReady() method - should also not affect health status
	err = pipelineWatcher.NotReady()
	require.NoError(t, err, "NotReady() should not return an error")

	// Health status should still be unavailable
	resp, err = client.Get(fmt.Sprintf("http://localhost:%d/", port))
	require.NoError(t, err)
	assert.Equal(t, "503 Service Unavailable", resp.Status, "Health endpoint should remain unavailable")
}
