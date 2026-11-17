// Copyright The OpenTelemetry Authors
// Copyright 2026-present Datadog, Inc.
// SPDX-License-Identifier: Apache-2.0

package remoteconfig

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	configmock "github.com/DataDog/datadog-agent/pkg/config/mock"
	configmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	rcapi "github.com/DataDog/datadog-agent/pkg/config/remote/api"
	rcservice "github.com/DataDog/datadog-agent/pkg/config/remote/service"
	pbgo "github.com/DataDog/datadog-agent/pkg/proto/pbgo/core"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var Type = component.MustNewType("datadog")

const tracerRequest = `{"client":{"id":"client-one","is_tracer":true,"state":{"root_version":1},"products":["APM_TRACING"],"client_tracer":{"language":"java","tracer_version":"1.68.1","runtime_id":"runtime-one","service":"rc-poc","env":"test"}}}`

type fakeService struct {
	started atomic.Bool
	stopped atomic.Bool
	get     func(context.Context, *pbgo.ClientGetConfigsRequest) (*pbgo.ClientGetConfigsResponse, error)
}

func (f *fakeService) Start() { f.started.Store(true) }
func (f *fakeService) Stop() error {
	f.stopped.Store(true)
	return nil
}

func (f *fakeService) ClientGetConfigs(ctx context.Context, r *pbgo.ClientGetConfigsRequest) (*pbgo.ClientGetConfigsResponse, error) {
	if f.get != nil {
		return f.get(ctx, r)
	}
	return &pbgo.ClientGetConfigsResponse{}, nil
}

func testExtension(t *testing.T, service *fakeService) *remoteConfigExtension {
	t.Helper()
	cfg := DefaultConfig()
	cfg.ServerConfig.NetAddr.Endpoint = "127.0.0.1:0"
	cfg.API.Key = "collector-secret"
	cfg.CacheDirectory = t.TempDir()
	return &remoteConfigExtension{
		config: cfg, settings: extension.Settings{ID: component.NewID(Type), TelemetrySettings: componenttest.NewNopTelemetrySettings()},
		service:    service,
		newService: func(context.Context, component.Host) (remoteConfigService, error) { return service, nil },
	}
}

func TestAgentProtocol(t *testing.T) {
	targets := []byte(`{"signed":{"version":2},"signatures":[]}`)
	want := &pbgo.ClientGetConfigsResponse{
		Roots: [][]byte{[]byte("signed-root")}, Targets: targets,
		TargetFiles:   []*pbgo.File{{Path: "datadog/1/APM_TRACING/rule/hash", Raw: []byte(`{"sample_rate":0.25}`)}},
		ClientConfigs: []string{"datadog/1/APM_TRACING/rule/hash"},
	}
	service := &fakeService{get: func(ctx context.Context, r *pbgo.ClientGetConfigsRequest) (*pbgo.ClientGetConfigsResponse, error) {
		require.Equal(t, "java", r.Client.ClientTracer.Language)
		require.Equal(t, "client-one", r.Client.Id)
		require.Equal(t, []string{"APM_TRACING"}, r.Client.Products)
		_, deadline := ctx.Deadline()
		require.True(t, deadline)
		return want, nil
	}}
	e := testExtension(t, service)
	w := httptest.NewRecorder()
	e.serveHTTP(w, httptest.NewRequest(http.MethodPost, "/v0.7/config", strings.NewReader(tracerRequest)))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var got pbgo.ClientGetConfigsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, want.Roots, got.Roots)
	require.Equal(t, targets, got.Targets)
	require.Equal(t, want.TargetFiles[0].Raw, got.TargetFiles[0].Raw)
	require.Equal(t, want.ClientConfigs, got.ClientConfigs)
	require.NotContains(t, w.Body.String(), "collector-secret")
}

func TestDiscovery(t *testing.T) {
	e := testExtension(t, &fakeService{})
	w := httptest.NewRecorder()
	e.serveHTTP(w, httptest.NewRequest(http.MethodGet, "/info", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code)
	var info struct {
		Endpoints []string `json:"endpoints"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &info))
	require.Equal(t, []string{"/v0.7/config"}, info.Endpoints)
	require.NotContains(t, w.Body.String(), "collector-secret")
}

func TestRejectedRequestsDoNotReachService(t *testing.T) {
	for _, tt := range []struct {
		name, method, path, body string
		code                     int
	}{
		{"unknown", "POST", "/v0.4/traces", tracerRequest, 404},
		{"wrong method", "GET", "/v0.7/config", tracerRequest, 405},
		{"query", "POST", "/v0.7/config?site=other", tracerRequest, 400},
		{"bad json", "POST", "/v0.7/config", "{", 400},
		{"trailing json", "POST", "/v0.7/config", tracerRequest + " {}", 400},
		{"not a tracer", "POST", "/v0.7/config", `{"client":{"is_agent":true}}`, 400},
		{"null", "POST", "/v0.7/config", "null", 400},
		{"oversize", "POST", "/v0.7/config", strings.Repeat(" ", 1025), 413},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := testExtension(t, &fakeService{get: func(context.Context, *pbgo.ClientGetConfigsRequest) (*pbgo.ClientGetConfigsResponse, error) {
				t.Fatal("invalid request reached the RC service")
				return nil, nil
			}})
			e.config.ServerConfig.MaxRequestBodySize = 1024
			w := httptest.NewRecorder()
			e.serveHTTP(w, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
			require.Equal(t, tt.code, w.Code)
		})
	}
}

func TestServiceErrors(t *testing.T) {
	for _, code := range []codes.Code{codes.InvalidArgument, codes.DeadlineExceeded, codes.Unavailable, codes.Internal} {
		t.Run(code.String(), func(t *testing.T) {
			e := testExtension(t, &fakeService{get: func(context.Context, *pbgo.ClientGetConfigsRequest) (*pbgo.ClientGetConfigsResponse, error) {
				return nil, status.Error(code, "sensitive backend detail")
			}})
			w := httptest.NewRecorder()
			e.serveHTTP(w, httptest.NewRequest(http.MethodPost, "/v0.7/config", strings.NewReader(tracerRequest)))
			want := map[codes.Code]int{codes.InvalidArgument: 400, codes.DeadlineExceeded: 504, codes.Unavailable: 503, codes.Internal: 500}
			require.Equal(t, want[code], w.Code)
			require.NotContains(t, w.Body.String(), "sensitive")
		})
	}
}

func TestLifecycle(t *testing.T) {
	service := &fakeService{}
	e := testExtension(t, service)
	require.NoError(t, e.Shutdown(t.Context()))
	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	require.True(t, service.started.Load())
	response, err := http.Get("http://" + e.listener.Addr().String() + "/info")
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, 200, response.StatusCode)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, e.Shutdown(ctx))
	require.True(t, service.stopped.Load())
	require.NoError(t, e.Shutdown(ctx))
}

func TestFailedStartReleasesListener(t *testing.T) {
	e := testExtension(t, &fakeService{})
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	endpoint := l.Addr().String()
	require.NoError(t, l.Close())
	e.config.ServerConfig.NetAddr.Endpoint = endpoint
	e.newService = func(context.Context, component.Host) (remoteConfigService, error) {
		return nil, errors.New("cache unavailable")
	}
	require.ErrorContains(t, e.Start(t.Context(), componenttest.NewNopHost()), "cache unavailable")
	l, err = net.Listen("tcp", endpoint)
	require.NoError(t, err)
	require.NoError(t, l.Close())
}

func TestConfig(t *testing.T) {
	cfg := DefaultConfig()
	require.ErrorContains(t, ValidateConfig(cfg), "api.key")
	cfg.API.Key = "collector-secret"
	require.ErrorContains(t, ValidateConfig(cfg), "cache_directory")
	cfg.CacheDirectory = t.TempDir()
	require.NoError(t, ValidateConfig(cfg))
	cfg.API.Site = "datadoghq.com/other"
	require.ErrorContains(t, ValidateConfig(cfg), "api.site")
}

func TestCollectorConfigDecoding(t *testing.T) {
	cfg := DefaultConfig()
	cfg.API = APIConfig{Key: "collector-secret", Site: "datadoghq.eu"}
	conf := confmap.NewFromStringMap(map[string]any{
		"endpoint": "127.0.0.1:9234",

		"cache_directory":  t.TempDir(),
		"refresh_interval": "5s",
	})
	require.NoError(t, conf.Unmarshal(cfg))
	require.NoError(t, ValidateConfig(cfg))
	require.Equal(t, "127.0.0.1:9234", cfg.ServerConfig.NetAddr.Endpoint)
	require.Equal(t, "datadoghq.eu", cfg.API.Site)
	require.Equal(t, 5*time.Second, cfg.RefreshInterval)
}

func TestAgentNormalization(t *testing.T) {
	e := testExtension(t, &fakeService{get: func(_ context.Context, r *pbgo.ClientGetConfigsRequest) (*pbgo.ClientGetConfigsResponse, error) {
		require.Equal(t, "bad_service", r.Client.ClientTracer.Service)
		require.Equal(t, "prod_canary", r.Client.ClientTracer.Env)
		return &pbgo.ClientGetConfigsResponse{}, nil
	}})
	body := strings.ReplaceAll(strings.ReplaceAll(tracerRequest, "rc-poc", "bad$service"), `"env":"test"`, `"env":"PROD Canary"`)
	w := httptest.NewRecorder()
	e.serveHTTP(w, httptest.NewRequest(http.MethodPost, "/v0.7/config", strings.NewReader(body)))
	require.Equal(t, 200, w.Code)
}

func TestShutdownCancelsRealServiceBackend(t *testing.T) {
	entered := make(chan struct{})
	var once sync.Once
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v0.1/org" {
			once.Do(func() { close(entered) })
			<-r.Context().Done()
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)
	cfg := configmock.New(t)
	cfg.Set("remote_configuration.no_tls", true, configmodel.SourceAgentRuntime)
	u, err := url.Parse(backend.URL)
	require.NoError(t, err)
	api, err := rcapi.NewHTTPClient(rcapi.Auth{APIKey: "collector-secret"}, cfg, u)
	require.NoError(t, err)
	e := testExtension(t, &fakeService{})
	reporter, err := newServiceReporter(e.settings)
	require.NoError(t, err)
	e.newService = func(ctx context.Context, _ component.Host) (remoteConfigService, error) {
		return rcservice.NewService(cfg, "DDOT test", backend.URL, "test-host", func() []string { return nil }, reporter, "7.87.0",
			rcservice.WithAPIKey("collector-secret"), rcservice.WithDatabasePath(t.TempDir()),
			rcservice.WithAPIClient(boundedAPI{API: api, lifetime: ctx, timeout: time.Minute}))
	}
	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, e.Shutdown(t.Context())) })
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("real service did not request organization data")
	}
	done := make(chan error, 1)
	go func() { done <- e.Shutdown(t.Context()) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not cancel the stalled backend request")
	}
}

func TestHTTPDeadlineBoundsServiceThatIgnoresCancellation(t *testing.T) {
	release := make(chan struct{})
	finished := make(chan struct{})
	e := testExtension(t, &fakeService{get: func(context.Context, *pbgo.ClientGetConfigsRequest) (*pbgo.ClientGetConfigsResponse, error) {
		defer close(finished)
		<-release
		return &pbgo.ClientGetConfigsResponse{}, nil
	}})
	e.config.RequestTimeout = 50 * time.Millisecond
	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { close(release); <-finished; require.NoError(t, e.Shutdown(t.Context())) }()
	client := &http.Client{Timeout: time.Second}
	start := time.Now()
	response, err := client.Post("http://"+e.listener.Addr().String()+"/v0.7/config", "application/json", strings.NewReader(tracerRequest))
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
	require.Less(t, time.Since(start), time.Second)
}
