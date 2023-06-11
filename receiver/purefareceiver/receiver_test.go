// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package purefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver"

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"
)

func TestReceiverArray(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{}

	once := &sync.Once{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data []byte
		var err error

		once.Do(func() {
			data, err = os.ReadFile("testdata/array.txt")
			require.NoError(t, err)
			wg.Done()
		})

		fmt.Println("Received request:", r.URL)

		_, err = w.Write(data)
		require.NoError(t, err)
		fmt.Println(string(data))

	}))
	defer ts.Close()

	cfg, ok := createDefaultConfig().(*Config)
	require.True(t, ok)

	cfg.Endpoint = ts.URL
	cfg.Array = []internal.ScraperConfig{{
		Address: "array01",
	}}
	cfg.Settings = &Settings{
		ReloadIntervals: &ReloadIntervals{
			Array: 10 * time.Millisecond,
		},
	}

	sink := &consumertest.MetricsSink{}
	recv := newReceiver(cfg, receivertest.NewNopCreateSettings(), sink)
	wg.Add(1)

	// test
	err := recv.Start(context.Background(), componenttest.NewNopHost())
	wg.Wait()

	// verify
	assert.NoError(t, err)
	// assert.Greater(t, len(sink.AllMetrics()), 0, "expected to have received more than 0 metrics")
	// require.Equal(t, len(sink.AllMetrics()), 0)
	assert.Eventually(t, func() bool {
		return len(sink.AllMetrics()) == 1
	}, 10*time.Second, 10*time.Millisecond)

}

func TestStart(t *testing.T) {
	// prepare
	cfg, ok := createDefaultConfig().(*Config)
	require.True(t, ok)

	sink := &consumertest.MetricsSink{}
	recv := newReceiver(cfg, receivertest.NewNopCreateSettings(), sink)

	// test
	err := recv.Start(context.Background(), componenttest.NewNopHost())

	// verify
	assert.NoError(t, err)
}

func TestShutdown(t *testing.T) {
	// prepare
	cfg, ok := createDefaultConfig().(*Config)
	require.True(t, ok)

	sink := &consumertest.MetricsSink{}
	recv := newReceiver(cfg, receivertest.NewNopCreateSettings(), sink)

	err := recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// test
	err = recv.Shutdown(context.Background())

	// verify
	assert.NoError(t, err)
}
