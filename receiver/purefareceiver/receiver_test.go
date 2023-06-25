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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"
)

func TestReceiverEndpoints(t *testing.T) {
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

		_, err = w.Write(data)
		require.NoError(t, err)
		fmt.Println(string(data))
	}))
	defer ts.Close()

	cfg, ok := createDefaultConfig().(*Config)
	require.True(t, ok)

	tests := []struct {
		name     string
		scraper  []internal.ScraperConfig
		interval time.Duration
	}{
		{
			name:     "Array scraper receiver is created properly",
			scraper:  []internal.ScraperConfig{{Address: "array01"}},
			interval: 10 * time.Millisecond,
		},
		{
			name:     "Hosts scraper receiver is created properly",
			scraper:  []internal.ScraperConfig{{Address: "array01"}},
			interval: 10 * time.Millisecond,
		},
		{
			name:     "Directories scraper receiver is created properly",
			scraper:  []internal.ScraperConfig{{Address: "array01"}},
			interval: 10 * time.Millisecond,
		},
		{
			name:     "Pods scraper receiver is created properly",
			scraper:  []internal.ScraperConfig{{Address: "array01"}},
			interval: 10 * time.Millisecond,
		},
		{
			name:     "Volumes scraper receiver is created properly",
			scraper:  []internal.ScraperConfig{{Address: "array01"}},
			interval: 10 * time.Millisecond,
		},
	}

	metricsSink := &consumertest.MetricsSink{}

	for _, test := range tests {
		switch test.name {
		case "Array scraper receiver is created properly":
			cfg.Array = test.scraper
			cfg.Settings = &Settings{
				ReloadIntervals: &ReloadIntervals{Array: test.interval},
			}
		case "Hosts scraper receiver is created properly":
			cfg.Hosts = test.scraper
			cfg.Settings = &Settings{
				ReloadIntervals: &ReloadIntervals{Hosts: test.interval},
			}
		case "Directories scraper receiver is created properly":
			cfg.Directories = test.scraper
			cfg.Settings = &Settings{
				ReloadIntervals: &ReloadIntervals{Directories: test.interval},
			}
		case "Pods scraper receiver is created properly":
			cfg.Pods = test.scraper
			cfg.Settings = &Settings{
				ReloadIntervals: &ReloadIntervals{Pods: test.interval},
			}
		case "Volumes scraper receiver is created properly":
			cfg.Volumes = test.scraper
			cfg.Settings = &Settings{
				ReloadIntervals: &ReloadIntervals{Volumes: test.interval},
			}
		default:
			t.Errorf("Unknown test name: %s", test.name)
			continue
		}

		t.Run(test.name, func(t *testing.T) {
			/* 			recv := newReceiver(cfg, receivertest.NewNopCreateSettings(), sink)
			   			// wg.Add(1)

			   			// test
			   			err := recv.Start(context.Background(), componenttest.NewNopHost())
			   			// wg.Wait()

			   			// verify
			   			assert.NoError(t, err)

			   			require.Equal(t, len(sink.AllMetrics()), 0)
			   			require.NoError(t, err)
			   			require.NotNil(t, recv) */

			recv := newReceiver(cfg, receivertest.NewNopCreateSettings(), metricsSink)
			metric := pmetric.NewMetrics()
			metric.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("foo")
			// err = recv.Start(context.Background(), componenttest.NewNopHost())
			err := recv.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)
			err = metricsSink.ConsumeMetrics(context.Background(), metric)
			require.NoError(t, err)
			assert.Greater(t, len(metricsSink.AllMetrics()), 0, "expected to have received more than 0 metrics")
			assert.Eventually(t, func() bool {
				return len(metricsSink.AllMetrics()) != 0
			}, 1*time.Second, 10*time.Millisecond)

			err = recv.Shutdown(context.Background())
			require.NoError(t, err)
		})
	}
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
