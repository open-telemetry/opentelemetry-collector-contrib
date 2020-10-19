// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !windows
// TODO review if tests should succeed on Windows

package dockerstatsreceiver

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	dtypes "github.com/docker/docker/api/types"
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
	cli, err := newDockerClient(config, zap.NewNop())
	assert.Nil(t, cli)
	require.Error(t, err)
	assert.Equal(t, "could not create docker client: unable to parse docker host `$notavalidendpoint*`", err.Error())
}

func TestInvalidExclude(t *testing.T) {
	config := NewFactory().CreateDefaultConfig().(*Config)
	config.ExcludedImages = []string{"["}
	cli, err := newDockerClient(config, zap.NewNop())
	assert.Nil(t, cli)
	require.Error(t, err)
	assert.Equal(t, "could not determine docker client excluded images: invalid glob item: unexpected end of input", err.Error())
}

func tmpSock(t *testing.T) (net.Listener, string) {
	f, err := ioutil.TempFile(os.TempDir(), "testsock")
	if err != nil {
		t.Fatal(err)
	}
	addr := f.Name()
	os.Remove(addr)

	listener, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}

	return listener, addr
}

func expectedConnectError(addr string) string {
	return fmt.Sprintf("Cannot connect to the Docker daemon at unix://%s.", addr)
}

func TestWatchingTimeouts(t *testing.T) {
	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		Timeout:  50 * time.Millisecond,
	}

	cli, err := newDockerClient(config, zap.NewNop())
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedError := expectedConnectError(addr)

	shouldHaveTaken := time.Now().Add(100 * time.Millisecond).UnixNano()

	err = cli.LoadContainerList(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedError)

	observed, logs := observer.New(zapcore.WarnLevel)
	cli, err = newDockerClient(config, zap.New(observed))
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	cnt, ofInterest := cli.inspectedContainerIsOfInterest(context.Background(), "SomeContainerId")
	assert.False(t, ofInterest)
	assert.Nil(t, cnt)
	assert.Equal(t, 1, len(logs.All()))
	for _, l := range logs.All() {
		assert.Contains(t, l.ContextMap()["error"], expectedError)
	}

	assert.GreaterOrEqual(
		t, time.Now().UnixNano(), shouldHaveTaken,
		"Client timeouts don't appear to have been exercised.",
	)
}

func TestFetchingTimeouts(t *testing.T) {
	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		Timeout:  50 * time.Millisecond,
	}

	cli, err := newDockerClient(config, zap.NewNop())
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedError := expectedConnectError(addr)

	shouldHaveTaken := time.Now().Add(50 * time.Millisecond).UnixNano()

	observed, logs := observer.New(zapcore.WarnLevel)
	cli, err = newDockerClient(config, zap.New(observed))
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	md, err := cli.FetchContainerStatsAndConvertToMetrics(
		context.Background(),
		DockerContainer{
			ContainerJSON: &dtypes.ContainerJSON{
				ContainerJSONBase: &dtypes.ContainerJSONBase{
					ID: "notARealContainerId",
				},
			},
		},
	)

	assert.Nil(t, md)
	require.Error(t, err)

	assert.Contains(t, err.Error(), expectedError)

	assert.Equal(t, 1, len(logs.All()))
	for _, l := range logs.All() {
		assert.Contains(t, l.ContextMap()["error"], expectedError)
	}

	assert.GreaterOrEqual(
		t, time.Now().UnixNano(), shouldHaveTaken,
		"Client timeouts don't appear to have been exercised.",
	)

}

func TestToStatsJSONErrorHandling(t *testing.T) {
	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		Timeout:  50 * time.Millisecond,
	}

	cli, err := newDockerClient(config, zap.NewNop())
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	dc := &DockerContainer{
		ContainerJSON: &dtypes.ContainerJSON{
			ContainerJSONBase: &dtypes.ContainerJSONBase{
				ID: "notARealContainerId",
			},
		},
	}

	statsJSON, err := cli.toStatsJSON(
		dtypes.ContainerStats{
			Body: ioutil.NopCloser(strings.NewReader("")),
		}, dc,
	)
	assert.Nil(t, statsJSON)
	assert.Equal(t, io.EOF, err)

	statsJSON, err = cli.toStatsJSON(
		dtypes.ContainerStats{
			Body: ioutil.NopCloser(strings.NewReader("{\"Networks\": 123}")),
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
		w.Write([]byte{})
	}))
	defer srv.Close()

	observed, logs := observer.New(zapcore.WarnLevel)
	config := &Config{
		Endpoint: srv.URL,
		Timeout:  50 * time.Millisecond,
	}

	cli, err := newDockerClient(config, zap.New(observed))
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	go cli.ContainerEventLoop(context.Background())

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
