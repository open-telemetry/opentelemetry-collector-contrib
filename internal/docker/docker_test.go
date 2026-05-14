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

	ctypes "github.com/moby/moby/api/types/container"
	docker "github.com/moby/moby/client"
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
	defer cli.Close()

	expectedError := "context deadline exceeded"

	shouldHaveTaken := time.Now().Add(100 * time.Millisecond).UnixNano()

	err = cli.LoadContainerList(t.Context())
	assert.ErrorContains(t, err, expectedError)
	observed, logs := observer.New(zapcore.WarnLevel)
	cli.Close() // close the first one
	cli, err = NewDockerClient(config, zap.New(observed))
	assert.NotNil(t, cli)
	assert.NoError(t, err)
	defer cli.Close()

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
	defer cli.Close()

	expectedError := "context deadline exceeded"

	shouldHaveTaken := time.Now().Add(50 * time.Millisecond).UnixNano()

	observed, logs := observer.New(zapcore.WarnLevel)
	cli.Close()
	cli, err = NewDockerClient(config, zap.New(observed))
	assert.NotNil(t, cli)
	assert.NoError(t, err)
	defer cli.Close()

	statsJSON, err := cli.FetchContainerStatsAsJSON(
		t.Context(),
		Container{
			InspectResponse: &ctypes.InspectResponse{
				ID: "notARealContainerId",
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
		InspectResponse: &ctypes.InspectResponse{
			ID: "notARealContainerId",
		},
	}

	statsJSON, err := cli.toStatsJSON(
		docker.ContainerStatsResult{
			Body: io.NopCloser(strings.NewReader("")),
		}, dc,
	)
	assert.Nil(t, statsJSON)
	assert.Equal(t, io.EOF, err)

	statsJSON, err = cli.toStatsJSON(
		docker.ContainerStatsResult{
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
		InspectResponse: &ctypes.InspectResponse{ID: "test"},
	})
	require.NoError(t, err, "should not fail with context canceled")
	require.NotNil(t, stats)
}

// TestStatsStreamUpdatesLatestStats verifies that startContainerStream populates
// LatestContainerStats even when the server delays before sending the first frame.
func TestStatsStreamUpdatesLatestStats(t *testing.T) {
	const containerID = "testContainer"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			w.Header().Set("Content-Type", "application/json")
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
			_, _ = w.Write([]byte(`{"cpu_stats":{"cpu_usage":{"total_usage":100}},"memory_stats":{}}`))
		}
	}))
	defer srv.Close()

	cli, err := NewDockerClient(&Config{Endpoint: srv.URL, Timeout: 5 * time.Second}, zap.NewNop())
	require.NoError(t, err)
	defer cli.Close()

	_, cancel := context.WithCancel(t.Context())
	defer cancel()

	cli.startContainerStream(containerID)

	require.Eventually(t, func() bool {
		_, ok := cli.LatestContainerStats(containerID, 0)
		return ok
	}, 5*time.Second, 10*time.Millisecond, "timed out waiting for stats")

	stats, _ := cli.LatestContainerStats(containerID, 0)
	require.NotNil(t, stats)
	assert.Equal(t, uint64(100), stats.CPUStats.CPUUsage.TotalUsage)
}

// TestStatsStreamHandlesInvalidJSON verifies that the stream goroutine logs a warning
// and leaves LatestContainerStats empty when the server sends undecodable JSON.
func TestStatsStreamHandlesInvalidJSON(t *testing.T) {
	const containerID = "testContainer"
	observed, logs := observer.New(zapcore.WarnLevel)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Networks": 123}`))
		}
	}))
	defer srv.Close()

	cli, err := NewDockerClient(&Config{Endpoint: srv.URL, Timeout: 5 * time.Second}, zap.New(observed))
	require.NoError(t, err)
	defer cli.Close()

	_, cancel := context.WithCancel(t.Context())
	defer cancel()

	cli.startContainerStream(containerID)

	require.Eventually(t, func() bool {
		for _, l := range logs.All() {
			if strings.Contains(l.Message, "Error reading stats stream") {
				return true
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond, "expected warning about invalid JSON")

	_, ok := cli.LatestContainerStats(containerID, 0)
	assert.False(t, ok, "no valid stats should be available after decode error")
}

// TestLatestContainerStatsMaxAge verifies that LatestContainerStats respects the maxAge threshold.
func TestLatestContainerStatsMaxAge(t *testing.T) {
	const containerID = "testContainer"
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	cli, err := NewDockerClient(&Config{Endpoint: srv.URL, Timeout: 5 * time.Second}, zap.NewNop())
	require.NoError(t, err)
	defer cli.Close()

	freshStats := &cachedStats{
		stats:    &ctypes.StatsResponse{},
		recorded: time.Now(),
	}
	staleStats := &cachedStats{
		stats:    &ctypes.StatsResponse{},
		recorded: time.Now().Add(-10 * time.Second),
	}

	t.Run("no expiry when maxAge is zero", func(t *testing.T) {
		cli.streamLatestStats.Store(containerID, staleStats)
		_, ok := cli.LatestContainerStats(containerID, 0)
		assert.True(t, ok)
	})

	t.Run("fresh entry within maxAge is returned", func(t *testing.T) {
		cli.streamLatestStats.Store(containerID, freshStats)
		_, ok := cli.LatestContainerStats(containerID, 5*time.Second)
		assert.True(t, ok)
	})

	t.Run("stale entry beyond maxAge is a miss", func(t *testing.T) {
		cli.streamLatestStats.Store(containerID, staleStats)
		_, ok := cli.LatestContainerStats(containerID, 5*time.Second)
		assert.False(t, ok)
	})

	t.Run("missing entry is a miss", func(t *testing.T) {
		cli.streamLatestStats.Delete(containerID)
		_, ok := cli.LatestContainerStats(containerID, 5*time.Second)
		assert.False(t, ok)
	})
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
	defer cli.Close()

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
	// moby/moby/client removed NegotiateAPIVersionPing, so version negotiation
	// can only happen via a real Ping call. We need a test server for that.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("API-Version", "1.45")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer srv.Close()

	config := &Config{
		Endpoint:         srv.URL,
		Timeout:          5 * time.Second,
		DockerAPIVersion: "", // empty triggers auto-negotiation
	}

	cli, err := NewDockerClient(config, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, cli)

	// Trigger version negotiation via Ping.
	_, err = cli.client.Ping(t.Context(), docker.PingOptions{NegotiateAPIVersion: true})
	require.NoError(t, err)
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
