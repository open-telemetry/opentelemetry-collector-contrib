// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckv2extension

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
	cfg := createDefaultConfig().(*Config)
	cfg.HTTPConfig.Endpoint = testutil.GetAvailableLocalAddress(t)
	cfg.GRPCConfig.NetAddr.Endpoint = testutil.GetAvailableLocalAddress(t)
	cfg.UseV2 = true
	ext := newExtension(context.Background(), *cfg, extensiontest.NewNopSettings(extensiontest.NopType))

	// Status before Start will be StatusNone
	st, ok := ext.aggregator.AggregateStatus(status.ScopeAll, status.Concise)
	require.True(t, ok)
	assert.Equal(t, componentstatus.StatusNone, st.Status())

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

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
	assert.Eventually(t, func() bool {
		st, ok = ext.aggregator.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(t, ok)
		return st.Status() == componentstatus.StatusStarting
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, ext.Ready())

	assert.Eventually(t, func() bool {
		st, ok = ext.aggregator.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(t, ok)
		return st.Status() == componentstatus.StatusOK
	}, time.Second, 10*time.Millisecond)

	// StatusStopping will be sent immediately.
	for _, id := range traces.InstanceIDs() {
		ext.ComponentStatusChanged(id, componentstatus.NewEvent(componentstatus.StatusStopping))
	}

	assert.Eventually(t, func() bool {
		st, ok = ext.aggregator.AggregateStatus(status.ScopeAll, status.Concise)
		require.True(t, ok)
		return st.Status() == componentstatus.StatusStopping
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, ext.NotReady())
	require.NoError(t, ext.Shutdown(context.Background()))

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

	cfg := createDefaultConfig().(*Config)
	cfg.UseV2 = true
	cfg.HTTPConfig.Endpoint = endpoint
	cfg.HTTPConfig.Config.Enabled = true
	cfg.HTTPConfig.Config.Path = "/config"

	ext := newExtension(context.Background(), *cfg, extensiontest.NewNopSettings(extensiontest.NopType))

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(context.Background())) })

	client := &http.Client{}
	url := fmt.Sprintf("http://%s/config", endpoint)

	var resp *http.Response

	resp, err = client.Get(url)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	require.NoError(t, ext.NotifyConfig(context.Background(), confMap))

	resp, err = client.Get(url)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, string(confJSON), string(body))
}

func TestShutdown(t *testing.T) {
	t.Run("error in http server start", func(t *testing.T) {
		// Want to get error in http server start
		endpoint := testutil.GetAvailableLocalAddress(t)
		l, err := net.Listen("tcp", endpoint)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, l.Close()) })

		cfg := createDefaultConfig().(*Config)
		cfg.UseV2 = true
		cfg.HTTPConfig.Endpoint = endpoint

		ext := newExtension(context.Background(), *cfg, extensiontest.NewNopSettings(extensiontest.NopType))
		// Get address already in use here
		require.Error(t, ext.Start(context.Background(), componenttest.NewNopHost()))

		require.NoError(t, ext.Shutdown(context.Background()))
	})
}
