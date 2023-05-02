// Copyright The OpenTelemetry Authors
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

//go:build !windows
// +build !windows

package podmanreceiver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type MockClient struct {
	PingF   func(context.Context) error
	StatsF  func(context.Context, url.Values) ([]containerStats, error)
	ListF   func(context.Context, url.Values) ([]container, error)
	EventsF func(context.Context, url.Values) (<-chan event, <-chan error)
}

func (c *MockClient) ping(ctx context.Context) error {
	return c.PingF(ctx)
}

func (c *MockClient) stats(ctx context.Context, options url.Values) ([]containerStats, error) {
	return c.StatsF(ctx, options)
}

func (c *MockClient) list(ctx context.Context, options url.Values) ([]container, error) {
	return c.ListF(ctx, options)
}

func (c *MockClient) events(ctx context.Context, options url.Values) (<-chan event, <-chan error) {
	return c.EventsF(ctx, options)
}

var baseClient = MockClient{
	PingF: func(context.Context) error {
		return nil
	},
	StatsF: func(context.Context, url.Values) ([]containerStats, error) {
		return nil, nil
	},
	ListF: func(context.Context, url.Values) ([]container, error) {
		return nil, nil
	},
	EventsF: func(context.Context, url.Values) (<-chan event, <-chan error) {
		return nil, nil
	},
}

func TestWatchingTimeouts(t *testing.T) {
	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		Timeout:  50 * time.Millisecond,
	}

	client, err := newLibpodClient(zap.NewNop(), config)
	assert.Nil(t, err)

	cli := newContainerScraper(client, zap.NewNop(), config)
	assert.NotNil(t, cli)

	expectedError := "context deadline exceeded"

	shouldHaveTaken := time.Now().Add(100 * time.Millisecond).UnixNano()

	err = cli.loadContainerList(context.Background())
	require.Error(t, err)

	container, err := cli.fetchContainerStats(context.Background(), container{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedError)
	assert.Empty(t, container)

	assert.GreaterOrEqual(
		t, time.Now().UnixNano(), shouldHaveTaken,
		"Client timeouts don't appear to have been exercised.",
	)
}

func TestEventLoopHandlesError(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(2) // confirm retry occurs

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/events") {
			wg.Done()
		}
		_, err := w.Write([]byte{})
		assert.NoError(t, err)
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	observed, logs := observer.New(zapcore.WarnLevel)
	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		Timeout:  50 * time.Millisecond,
	}

	client, err := newLibpodClient(zap.NewNop(), config)
	assert.Nil(t, err)

	cli := newContainerScraper(client, zap.New(observed), config)
	assert.NotNil(t, cli)

	go cli.containerEventLoop(context.Background())

	assert.Eventually(t, func() bool {
		for _, l := range logs.All() {
			assert.Contains(t, l.Message, "Error watching podman container events")
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

func TestEventLoopHandles(t *testing.T) {
	eventChan := make(chan event)
	errChan := make(chan error)

	eventClient := baseClient
	eventClient.EventsF = func(context.Context, url.Values) (<-chan event, <-chan error) {
		return eventChan, errChan
	}
	eventClient.ListF = func(context.Context, url.Values) ([]container, error) {
		return []container{{
			ID: "c1",
		}}, nil
	}

	cli := newContainerScraper(&eventClient, zap.NewNop(), &Config{})
	assert.NotNil(t, cli)

	assert.Equal(t, 0, len(cli.containers))

	go cli.containerEventLoop(context.Background())
	eventChan <- event{ID: "c1", Status: "start"}

	assert.Eventually(t, func() bool {
		cli.containersLock.Lock()
		defer cli.containersLock.Unlock()
		return assert.Equal(t, 1, len(cli.containers))
	}, 1*time.Second, 1*time.Millisecond, "failed to update containers list.")

	eventChan <- event{ID: "c1", Status: "died"}

	assert.Eventually(t, func() bool {
		cli.containersLock.Lock()
		defer cli.containersLock.Unlock()
		return assert.Equal(t, 0, len(cli.containers))
	}, 1*time.Second, 1*time.Millisecond, "failed to update containers list.")
}

func TestInspectAndPersistContainer(t *testing.T) {
	inspectClient := baseClient
	inspectClient.ListF = func(context.Context, url.Values) ([]container, error) {
		return []container{{
			ID: "c1",
		}}, nil
	}

	cli := newContainerScraper(&inspectClient, zap.NewNop(), &Config{})
	assert.NotNil(t, cli)

	assert.Equal(t, 0, len(cli.containers))

	stats, ok := cli.inspectAndPersistContainer(context.Background(), "c1")
	assert.True(t, ok)
	assert.NotNil(t, stats)
	assert.Equal(t, 1, len(cli.containers))
}
