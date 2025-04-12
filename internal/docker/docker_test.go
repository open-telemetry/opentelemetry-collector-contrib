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

	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	err = cli.LoadContainerList(context.Background())
	assert.ErrorContains(t, err, expectedError)
	observed, logs := observer.New(zapcore.WarnLevel)
	cli, err = NewDockerClient(config, zap.New(observed))
	assert.NotNil(t, cli)
	assert.NoError(t, err)

	cnt, ofInterest := cli.inspectedContainerIsOfInterest(context.Background(), "SomeContainerId")
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
		context.Background(),
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

	ctx, cancel := context.WithCancel(context.Background())
	go cli.ContainerEventLoop(ctx)
	defer cancel()

	assert.Eventually(t, func() bool {
		for _, l := range logs.All() {
			assert.Contains(t, l.Message, "Error watching docker container events")
			assert.Contains(t, l.ContextMap()["error"], "EOF")
		}
		return len(logs.All()) > 0
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
