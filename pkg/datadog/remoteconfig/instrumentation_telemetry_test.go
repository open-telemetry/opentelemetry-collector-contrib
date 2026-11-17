// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remoteconfig

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"
)

func telemetryServer(t *testing.T, handler http.HandlerFunc) *remoteConfigExtension {
	t.Helper()
	intake := httptest.NewTLSServer(handler)
	t.Cleanup(intake.Close)
	ca := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intake.Certificate().Raw}), 0o600))
	rc := DefaultConfig()
	rc.ServerConfig.NetAddr.Endpoint = "127.0.0.1:0"
	telemetry := DefaultInstrumentationTelemetryConfig()
	telemetry.Enabled, telemetry.Endpoint = true, intake.URL+"/api/v2/apmtelemetry"
	backend := confighttp.NewDefaultClientConfig()
	backend.TLS.CAFile = ca
	server, err := NewSDKServer(extension.Settings{ID: component.NewID(Type), TelemetrySettings: componenttest.NewNopTelemetrySettings()}, &SDKConfig{
		RemoteConfig: rc, InstrumentationTelemetry: telemetry,
		API: APIConfig{Key: "collector-secret", Site: "datadoghq.com"}, Backend: backend,
	}, nil)
	require.NoError(t, err)
	e := server.(*remoteConfigExtension)
	require.True(t, e.telemetryOnly)
	require.Empty(t, e.config.CacheDirectory, "telemetry must not require an RC cache")
	return e
}

func startTelemetryServer(t *testing.T, e *remoteConfigExtension) string {
	t.Helper()
	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, e.Shutdown(t.Context())) })
	return "http://" + e.listener.Addr().String()
}

func TestSDKTelemetryOpaqueForwardingWithoutRC(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "gzip"}[compressed], func(t *testing.T) {
			body := []byte(`{"api_version":"v2","request_type":"message-batch","runtime_id":"runtime-1","unknown":{"integer":1267650600228229401496703205376,"text":"preserve me"},"payload":[]}`)
			if compressed {
				var buffer bytes.Buffer
				writer := gzip.NewWriter(&buffer)
				_, err := writer.Write(body)
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				body = buffer.Bytes()
			}
			var requests atomic.Int32
			e := telemetryServer(t, func(w http.ResponseWriter, r *http.Request) {
				got, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(body, got) {
					t.Error("telemetry body changed in transit")
				}
				if r.URL.Path != "/api/v2/apmtelemetry" || r.Header.Get("DD-API-KEY") != "collector-secret" || r.Header.Get("Authorization") != "" || r.Header.Get("DD-APPLICATION-KEY") != "" {
					t.Error("incorrect destination or authentication")
				}
				if r.Header.Get("DD-Telemetry-Request-Type") != "message-batch" || r.Header.Get("DD-Client-Library-Language") != "python" || r.Header.Get("Datadog-Container-ID") != "container-1" {
					t.Error("SDK identity headers were lost")
				}
				if r.Header.Get("DD-Telemetry-Debug-Enabled") != "" {
					t.Error("connection-scoped header was forwarded")
				}
				if compressed && r.Header.Get("Content-Encoding") != "gzip" {
					t.Error("compression metadata was lost")
				}
				requests.Add(1)
				w.WriteHeader(http.StatusAccepted)
			})
			address := startTelemetryServer(t, e)
			req, err := http.NewRequest(http.MethodPost, address+telemetryPath, bytes.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("DD-API-KEY", "application-secret")
			req.Header.Set("DD-APPLICATION-KEY", "application-key")
			req.Header.Set("Authorization", "Bearer application-secret")
			req.Header.Set("DD-Telemetry-Request-Type", "message-batch")
			req.Header.Set("DD-Client-Library-Language", "python")
			req.Header.Set("Datadog-Container-ID", "container-1")
			req.Header.Set("Connection", "DD-Telemetry-Debug-Enabled")
			req.Header.Set("DD-Telemetry-Debug-Enabled", "true")
			if compressed {
				req.Header.Set("Content-Encoding", "gzip")
			}
			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.Equal(t, http.StatusAccepted, response.StatusCode)
			require.Equal(t, int32(1), requests.Load())
			require.Nil(t, e.service)
		})
	}
}

func TestSDKDiscoveryAndRouteIsolation(t *testing.T) {
	e := telemetryServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	address := startTelemetryServer(t, e)
	response, err := http.Get(address + "/info")
	require.NoError(t, err)
	var info struct{ Endpoints []string }
	require.NoError(t, json.NewDecoder(response.Body).Decode(&info))
	require.NoError(t, response.Body.Close())
	require.Equal(t, []string{"/telemetry/proxy/"}, info.Endpoints)
	for _, path := range []string{"/v0.7/config", "/telemetry/proxy/arbitrary"} {
		response, err = http.Post(address+path, "application/json", strings.NewReader(`{}`))
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusNotFound, response.StatusCode)
	}
	for _, test := range []struct {
		method, path string
		code         int
	}{
		{http.MethodGet, telemetryPath, http.StatusMethodNotAllowed},
		{http.MethodPost, telemetryPath + "?destination=other", http.StatusBadRequest},
	} {
		req := httptest.NewRequest(test.method, test.path, http.NoBody)
		w := httptest.NewRecorder()
		e.serveHTTP(w, req)
		require.Equal(t, test.code, w.Code)
	}
	rc := testExtension(t, &fakeService{})
	w := httptest.NewRecorder()
	rc.serveHTTP(w, httptest.NewRequest(http.MethodPost, telemetryPath, http.NoBody))
	require.Equal(t, http.StatusNotFound, w.Code)
	rc.telemetry = e.telemetry
	w = httptest.NewRecorder()
	rc.serveHTTP(w, httptest.NewRequest(http.MethodGet, "/info", http.NoBody))
	require.Contains(t, w.Body.String(), "/v0.7/config")
	require.Contains(t, w.Body.String(), "/telemetry/proxy/")
}

func TestSDKTelemetryDownstreamResults(t *testing.T) {
	for _, code := range []int{202, 400, 401, 403, 404, 429, 500, 503, 307} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var requests atomic.Int32
			e := telemetryServer(t, func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.Header().Set("Retry-After", "7")
				w.Header().Set("Location", "https://untrusted.invalid")
				w.WriteHeader(code)
				_, _ = io.WriteString(w, "collector-secret")
			})
			address := startTelemetryServer(t, e)
			response, err := http.Post(address+telemetryPath, "application/json", strings.NewReader(`{}`))
			require.NoError(t, err)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			want := code
			if code == 404 || code == 307 {
				want = http.StatusBadGateway
			}
			require.Equal(t, want, response.StatusCode)
			require.Equal(t, "7", response.Header.Get("Retry-After"))
			require.Empty(t, response.Header.Get("Location"))
			require.NotContains(t, string(body), "collector-secret")
			require.Equal(t, int32(1), requests.Load(), "the forwarder must not retry")
		})
	}
}

func TestSDKTelemetrySaturationAndShutdown(t *testing.T) {
	for _, limit := range []string{"concurrency", "bytes"} {
		t.Run(limit, func(t *testing.T) {
			entered := make(chan struct{})
			canceled := make(chan struct{})
			e := telemetryServer(t, func(_ http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(entered)
				<-r.Context().Done()
				close(canceled)
			})
			if limit == "concurrency" {
				e.telemetry.slots = make(chan struct{}, 1)
			} else {
				e.telemetry.config.MaxInflightBytes = 2
			}
			address := startTelemetryServer(t, e)
			done := make(chan struct{})
			go func() {
				defer close(done)
				response, err := http.Post(address+telemetryPath, "application/json", strings.NewReader(`{}`))
				if err == nil {
					_ = response.Body.Close()
				}
			}()
			<-entered
			response, err := http.Post(address+telemetryPath, "application/json", strings.NewReader(`{}`))
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
			require.Equal(t, "1", response.Header.Get("Retry-After"))
			shutdown, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			require.NoError(t, e.Shutdown(shutdown))
			<-done
			<-canceled
			require.Zero(t, e.telemetry.bytes.Load())
		})
	}
}

func TestSDKTelemetrySizeAndConfigurationLimits(t *testing.T) {
	e := telemetryServer(t, func(http.ResponseWriter, *http.Request) { t.Error("oversized request reached intake") })
	e.telemetry.config.MaxRequestBytes = 2
	address := startTelemetryServer(t, e)
	for _, knownLength := range []bool{false, true} {
		req, err := http.NewRequest(http.MethodPost, address+telemetryPath, strings.NewReader(`{"too":"large"}`))
		require.NoError(t, err)
		if !knownLength {
			req.ContentLength = -1
		}
		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusRequestEntityTooLarge, response.StatusCode)
	}
	c := DefaultInstrumentationTelemetryConfig()
	c.Enabled = true
	for _, bad := range []string{"http://example.com/api/v2/apmtelemetry", "https://example.com/other", "https://key@example.com/api/v2/apmtelemetry", "https://example.com/api/v2/apmtelemetry?q=x"} {
		c.Endpoint = bad
		require.Error(t, c.validate("datadoghq.com"))
	}
	c.Endpoint = ""
	c.MaxConcurrentRequests = 0
	require.Error(t, c.validate("datadoghq.com"))
	c = DefaultInstrumentationTelemetryConfig()
	c.Timeout = -time.Second
	require.Error(t, c.validate("datadoghq.com"))
}

func TestSDKTelemetryDeadline(t *testing.T) {
	canceled := make(chan struct{})
	e := telemetryServer(t, func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
		close(canceled)
	})
	e.telemetry.config.Timeout = 30 * time.Millisecond
	address := startTelemetryServer(t, e)
	client := &http.Client{Timeout: time.Second}
	response, err := client.Post(address+telemetryPath, "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Contains(t, []int{http.StatusBadGateway, http.StatusServiceUnavailable}, response.StatusCode)
	<-canceled
}
