// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

func TestHTTPServerPush(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	sink := new(consumertest.ProfilesSink)
	srvCfg := confighttp.NewDefaultServerConfig()
	srvCfg.NetAddr = confignet.AddrConfig{
		Endpoint:  addr,
		Transport: confignet.TransportTypeTCP,
	}

	srv := &HTTPServer{
		ServerConfig: srvCfg,
		Consumer:     sink,
		Settings:     receivertest.NewNopSettings(metadata.Type),
	}
	require.NoError(t, srv.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, srv.Shutdown(context.WithoutCancel(t.Context())))
	})

	pprofBody := generatePprofBody(t)

	url := fmt.Sprintf("http://%s%s", addr, PushPath)

	doRequest := func(method string, body []byte) *http.Response {
		req, reqErr := http.NewRequestWithContext(t.Context(), method, url, bytes.NewReader(body))
		require.NoError(t, reqErr)
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/octet-stream")
		}
		resp, doErr := http.DefaultClient.Do(req)
		require.NoError(t, doErr)
		return resp
	}

	// invalid method returns 405
	require.Eventually(t, func() bool {
		resp := doRequest(http.MethodGet, nil)
		resp.Body.Close()
		return resp.StatusCode == http.StatusMethodNotAllowed
	}, 3*time.Second, 50*time.Millisecond)

	// valid push returns 204
	resp := doRequest(http.MethodPost, pprofBody)
	resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.NotEmpty(t, sink.AllProfiles(), "expected profiles to be consumed")

	// invalid body returns 400
	resp = doRequest(http.MethodPost, []byte("not pprof"))
	resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHTTPServerPush_BodySizeLimit(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	srvCfg := confighttp.NewDefaultServerConfig()
	srvCfg.NetAddr = confignet.AddrConfig{
		Endpoint:  addr,
		Transport: confignet.TransportTypeTCP,
	}
	srvCfg.MaxRequestBodySize = 64

	srv := &HTTPServer{
		ServerConfig: srvCfg,
		Consumer:     new(consumertest.ProfilesSink),
		Settings:     receivertest.NewNopSettings(metadata.Type),
	}
	require.NoError(t, srv.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, srv.Shutdown(context.WithoutCancel(t.Context())))
	})

	url := fmt.Sprintf("http://%s%s", addr, PushPath)
	oversized := bytes.Repeat([]byte("x"), 1<<10)

	var resp *http.Response
	require.Eventually(t, func() bool {
		req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(oversized))
		if reqErr != nil {
			return false
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		var doErr error
		resp, doErr = http.DefaultClient.Do(req)
		return doErr == nil
	}, 3*time.Second, 50*time.Millisecond)
	resp.Body.Close()
	// confighttp returns 400 for oversized bodies; we surface 400 from profile parse failure.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func generatePprofBody(t *testing.T) []byte {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cpu.pprof")
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, pprof.StartCPUProfile(f))
	start := time.Now()
	x := 0
	for time.Since(start) < 100*time.Millisecond {
		for i := range 10000 {
			x = (x + i*i) % 1_000_003
		}
	}
	_ = x
	pprof.StopCPUProfile()
	require.NoError(t, f.Close())
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	return body
}
