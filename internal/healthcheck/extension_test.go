// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status/testhelpers"
)

func TestComponentStatus(t *testing.T) {
	cfg := NewDefaultConfig().(*Config)
	cfg.HTTPConfig.NetAddr.Endpoint = testutil.GetAvailableLocalAddress(t)
	cfg.GRPCConfig.NetAddr.Endpoint = testutil.GetAvailableLocalAddress(t)
	cfg.UseV2 = true
	ext := NewHealthCheckExtension(t.Context(), *cfg, extensiontest.NewNopSettings(extensiontest.NopType))
	defer func() {
		// Use Background context for shutdown in defer to avoid cancellation issues
		//nolint:usetesting // defer functions may run after test context is cancelled
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, ext.Shutdown(ctx))
	}()

	// Status before Start will be StatusNone
	st, ok := ext.aggregator.AggregateStatus(status.ScopeAll, status.Concise)
	require.True(t, ok)
	assert.Equal(t, componentstatus.StatusNone, st.Status())

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)

	// StatusStarting will be sent immediately.
	for _, id := range traces.InstanceIDs() {
		ext.ComponentStatusChanged(id, componentstatus.NewEvent(componentstatus.StatusStarting))
	}

	// StatusOK will be queued until the PipelineWatcher Ready method is called.
	for _, id := range traces.InstanceIDs() {
		ext.ComponentStatusChanged(id, componentstatus.NewEvent(componentstatus.StatusOK))
	}

	// Note the use of assert.Eventually here and throughout this test is because
	// status events are processed asynchronously in the background.
	assert.EventuallyWithT(t, func(tt *assert.CollectT) {
		st, ok = ext.aggregator.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(tt, ok)
		assert.Equal(tt, componentstatus.StatusStarting, st.Status())
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, ext.Ready())

	assert.EventuallyWithT(t, func(tt *assert.CollectT) {
		st, ok = ext.aggregator.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(tt, ok)
		assert.Equal(tt, componentstatus.StatusOK, st.Status())
	}, time.Second, 10*time.Millisecond)

	// StatusStopping will be sent immediately.
	for _, id := range traces.InstanceIDs() {
		ext.ComponentStatusChanged(id, componentstatus.NewEvent(componentstatus.StatusStopping))
	}

	assert.EventuallyWithT(t, func(tt *assert.CollectT) {
		st, ok = ext.aggregator.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(tt, ok)
		assert.Equal(tt, componentstatus.StatusStopping, st.Status())
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, ext.NotReady())
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	require.NoError(t, ext.Shutdown(ctx))

	// Events sent after shutdown will be discarded
	for _, id := range traces.InstanceIDs() {
		ext.ComponentStatusChanged(id, componentstatus.NewEvent(componentstatus.StatusStopped))
	}

	st, ok = ext.aggregator.AggregateStatus(status.ScopeAll, status.Concise)
	require.True(t, ok)
	assert.Equal(t, componentstatus.StatusStopping, st.Status())
}

func TestNotifyConfig(t *testing.T) {
	confMap, err := confmaptest.LoadConf(
		filepath.Join("internal", "http", "testdata", "config.yaml"),
	)
	require.NoError(t, err)
	confJSON, err := os.ReadFile(
		filepath.Clean(filepath.Join("internal", "http", "testdata", "config.json")),
	)
	require.NoError(t, err)

	endpoint := testutil.GetAvailableLocalAddress(t)

	cfg := NewDefaultConfig().(*Config)
	cfg.UseV2 = true
	cfg.HTTPConfig.NetAddr.Endpoint = endpoint
	cfg.HTTPConfig.Config.Enabled = true
	cfg.HTTPConfig.Config.Path = "/config"

	ext := NewHealthCheckExtension(t.Context(), *cfg, extensiontest.NewNopSettings(extensiontest.NopType))
	defer func() {
		// Use Background context for shutdown in defer to avoid cancellation issues
		//nolint:usetesting // defer functions may run after test context is cancelled
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, ext.Shutdown(ctx))
	}()

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

	client := &http.Client{}
	defer client.CloseIdleConnections()
	url := fmt.Sprintf("http://%s/config", endpoint)

	var resp *http.Response

	resp, err = client.Get(url)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	require.NoError(t, ext.NotifyConfig(t.Context(), confMap))

	resp, err = client.Get(url)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.JSONEq(t, string(confJSON), string(body))
}

func TestShutdown(t *testing.T) {
	t.Run("error in http server start", func(t *testing.T) {
		// Want to get error in http server start
		endpoint := testutil.GetAvailableLocalAddress(t)
		l, err := net.Listen("tcp", endpoint)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, l.Close()) })

		cfg := NewDefaultConfig().(*Config)
		cfg.UseV2 = true
		cfg.HTTPConfig.NetAddr.Endpoint = endpoint

		ext := NewHealthCheckExtension(t.Context(), *cfg, extensiontest.NewNopSettings(extensiontest.NopType))
		// Get address already in use here
		require.Error(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		require.NoError(t, ext.Shutdown(ctx))
	})
}
