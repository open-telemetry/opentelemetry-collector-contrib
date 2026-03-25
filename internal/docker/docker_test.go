// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	dtypes "github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestInvalidEndpoint(t *testing.T) {
	config := &Config{
		Endpoint: "$notavalidendpoint*",
	}
	cli, err := NewDockerClient(config, zap.NewNop())
	assert.Nil(t, cli)
	require.Error(t, err)
	assert.Equal(t, "could not create docker client: unable to parse docker host `$notavalidendpoint*`", err.Error())
}

func TestInvalidExclude(t *testing.T) {
	config := NewDefaultConfig()
	config.ExcludedImages = []string{"["}
	cli, err := NewDockerClient(config, zap.NewNop())
	assert.Nil(t, cli)
	require.Error(t, err)
	assert.Equal(t, "could not determine docker client excluded images: invalid glob item: unexpected end of input", err.Error())
}

func TestWatchingTimeouts(t *testing.T) {
	listener, addr := testListener(t)
	defer func() {
		assert.NoError(t, listener.Close())
	}()

	config := &Config{
		Endpoint: portableEndpoint(addr),
		Timeout:  50 * time.Millisecond,
	}

	cli, err := NewDockerClient(config, zap.NewNop())
	assert.NotNil(t, cli)
	assert.NoError(t, err)

	expectedError := "context deadline exceeded"

	shouldHaveTaken := time.Now().Add(100 * time.Millisecond).UnixNano()

	err = cli.LoadContainerList(t.Context())
	assert.ErrorContains(t, err, expectedError)
	observed, logs := observer.New(zapcore.WarnLevel)
	cli, err = NewDockerClient(config, zap.New(observed))
	assert.NotNil(t, cli)
	assert.NoError(t, err)

	cnt, ofInterest := cli.inspectedContainerIsOfInterest(t.Context(), "SomeContainerId")
	assert.False(t, ofInterest)
	assert.Nil(t, cnt)
	assert.Len(t, logs.All(), 1)
	for _, l := range logs.All() {
		assert.Contains(t, l.ContextMap()["error"], expectedError)
	}

	assert.GreaterOrEqual(
		t, time.Now().UnixNano(), shouldHaveTaken,
		"Client timeouts don't appear to have been exercised.",
	)
}

func TestFetchingTimeouts(t *testing.T) {
	listener, addr := testListener(t)

	defer func() {
		assert.NoError(t, listener.Close())
	}()

	config := &Config{
		Endpoint: portableEndpoint(addr),
		Timeout:  50 * time.Millisecond,
	}

	cli, err := NewDockerClient(config, zap.NewNop())
	assert.NotNil(t, cli)
	assert.NoError(t, err)

	expectedError := "context deadline exceeded"

	shouldHaveTaken := time.Now().Add(50 * time.Millisecond).UnixNano()

	observed, logs := observer.New(zapcore.WarnLevel)
	cli, err = NewDockerClient(config, zap.New(observed))
	assert.NotNil(t, cli)
	assert.NoError(t, err)

	statsJSON, err := cli.FetchContainerStatsAsJSON(
		t.Context(),
		Container{
			ContainerJSON: &ctypes.InspectResponse{
				ContainerJSONBase: &ctypes.ContainerJSONBase{
					ID: "notARealContainerId",
				},
			},
		},
	)

	assert.Nil(t, statsJSON)

	assert.ErrorContains(t, err, expectedError)

	assert.Len(t, logs.All(), 1)
	for _, l := range logs.All() {
		assert.Contains(t, l.ContextMap()["error"], expectedError)
	}

	assert.GreaterOrEqual(
		t, time.Now().UnixNano(), shouldHaveTaken,
		"Client timeouts don't appear to have been exercised.",
	)
}

func TestToStatsJSONErrorHandling(t *testing.T) {
	listener, addr := testListener(t)
	defer func() {
		assert.NoError(t, listener.Close())
	}()

	config := &Config{
		Endpoint: portableEndpoint(addr),
		Timeout:  50 * time.Millisecond,
	}

	cli, err := NewDockerClient(config, zap.NewNop())
	assert.NotNil(t, cli)
	assert.NoError(t, err)

	dc := &Container{
		ContainerJSON: &ctypes.InspectResponse{
			ContainerJSONBase: &ctypes.ContainerJSONBase{
				ID: "notARealContainerId",
			},
		},
	}

	statsJSON, err := cli.toStatsJSON(
		ctypes.StatsResponseReader{
			Body: io.NopCloser(strings.NewReader("")),
		}, dc,
	)
	assert.Nil(t, statsJSON)
	assert.Equal(t, io.EOF, err)

	statsJSON, err = cli.toStatsJSON(
		ctypes.StatsResponseReader{
			Body: io.NopCloser(strings.NewReader("{\"Networks\": 123}")),
		}, dc,
	)
	assert.Nil(t, statsJSON)
	require.Error(t, err)
}

func TestFetchContainerStatsAsJSONWithSlowResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			w.Header().Set("Content-Type", "application/json")
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
			_, _ = w.Write([]byte(`{"cpu_stats":{},"memory_stats":{}}`))
		}
	}))
	defer srv.Close()

	cli, err := NewDockerClient(&Config{Endpoint: srv.URL, Timeout: 5 * time.Second}, zap.NewNop())
	require.NoError(t, err)

	stats, err := cli.FetchContainerStatsAsJSON(t.Context(), Container{
		ContainerJSON: &ctypes.InspectResponse{ContainerJSONBase: &ctypes.ContainerJSONBase{ID: "test"}},
	})
	require.NoError(t, err, "should not fail with context canceled")
	require.NotNil(t, stats)
}

func TestEventLoopHandlesError(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(2) // confirm retry occurs
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/events") {
			wg.Done()
		}
		_, err := w.Write([]byte{})
		assert.NoError(t, err)
	}))
	defer srv.Close()

	observed, logs := observer.New(zapcore.WarnLevel)
	config := &Config{
		Endpoint: srv.URL,
		Timeout:  50 * time.Millisecond,
	}

	cli, err := NewDockerClient(config, zap.New(observed))
	assert.NotNil(t, cli)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	go cli.ContainerEventLoop(ctx)
	defer cancel()

	assert.EventuallyWithT(t, func(tt *assert.CollectT) {
		var eventErrLogs []observer.LoggedEntry
		for _, l := range logs.All() {
			if strings.Contains(l.Message, "Error watching docker container events") {
				eventErrLogs = append(eventErrLogs, l)
			}
		}
		if assert.NotEmpty(tt, eventErrLogs) {
			assert.Contains(tt, eventErrLogs[0].ContextMap()["error"], "EOF")
		}
	}, 1*time.Second, 1*time.Millisecond, "failed to find desired error logs.")

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		wg.Wait()
	}()
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("failed to retry events endpoint after error")
	case <-finished:
		return
	}
}

func portableEndpoint(addr string) string {
	endpoint := fmt.Sprintf("unix://%s", addr)
	if runtime.GOOS == "windows" {
		endpoint = fmt.Sprintf("npipe://%s", strings.ReplaceAll(addr, "\\", "/"))
	}
	return endpoint
}

func TestAPIVersionNegotiation(t *testing.T) {
	listener, addr := testListener(t)
	defer func() {
		assert.NoError(t, listener.Close())
	}()

	config := &Config{
		Endpoint:         portableEndpoint(addr),
		Timeout:          50 * time.Millisecond,
		DockerAPIVersion: "", // empty triggers auto-negotiation
	}

	cli, err := NewDockerClient(config, zap.NewNop())
	assert.NotNil(t, cli)
	assert.NoError(t, err)
	// Simulate a daemon ping response with API version "1.45".
	cli.client.NegotiateAPIVersionPing(dtypes.Ping{APIVersion: "1.45"})
	assert.Equal(t, "1.45", cli.client.ClientVersion())
}

func TestExplicitAPIVersion(t *testing.T) {
	listener, addr := testListener(t)
	defer func() {
		assert.NoError(t, listener.Close())
	}()

	config := &Config{
		Endpoint:         portableEndpoint(addr),
		Timeout:          50 * time.Millisecond,
		DockerAPIVersion: "1.44",
	}

	cli, err := NewDockerClient(config, zap.NewNop())
	assert.NotNil(t, cli)
	assert.NoError(t, err)
	assert.Equal(t, "1.44", cli.client.ClientVersion())
}

func TestDefaultConfigUsesNegotiation(t *testing.T) {
	config := NewDefaultConfig()
	assert.Empty(t, config.DockerAPIVersion, "Default config should have empty DockerAPIVersion for auto-negotiation")
}

func TestTLSClientConfig(t *testing.T) {
	// Start a TLS test server
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := &Config{
		Endpoint: srv.URL,
		Timeout:  5 * time.Second,
		TLS: configoptional.Some(configtls.ClientConfig{
			InsecureSkipVerify: true,
		}),
	}

	cli, err := NewDockerClient(config, zap.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, cli)
}

func TestUnauthenticatedTCPWarning(t *testing.T) {
	for _, endpoint := range []string{
		"tcp://192.168.1.1:2375",
		"http://192.168.1.1:2375",
		"TCP://192.168.1.1:2375",
		"HTTP://192.168.1.1:2375",
	} {
		t.Run(endpoint, func(t *testing.T) {
			observed, logs := observer.New(zapcore.WarnLevel)
			_, _ = NewDockerClient(&Config{Endpoint: endpoint}, zap.New(observed))
			assert.NotEmpty(t, logs.All(), "expected a deprecation warning for unauthenticated TCP endpoint")
			assert.Contains(t, logs.All()[0].Message, "deprecated")
		})
	}
}

func TestNoWarningForSecureEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"unix:///var/run/docker.sock",
		"npipe:////./pipe/docker_engine",
	} {
		t.Run(endpoint, func(t *testing.T) {
			observed, logs := observer.New(zapcore.WarnLevel)
			_, _ = NewDockerClient(&Config{Endpoint: endpoint}, zap.New(observed))
			assert.Empty(t, logs.All(), "expected no deprecation warning for non-TCP endpoint")
		})
	}
}

func TestTLSClientConfigInvalidCert(t *testing.T) {
	config := &Config{
		Endpoint: "https://example.com/",
		Timeout:  5 * time.Second,
		TLS: configoptional.Some(configtls.ClientConfig{
			Config: configtls.Config{
				CAFile: "/nonexistent/ca.pem",
			},
		}),
	}

	cli, err := NewDockerClient(config, zap.NewNop())
	assert.Nil(t, cli)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not load docker client TLS config")
}
