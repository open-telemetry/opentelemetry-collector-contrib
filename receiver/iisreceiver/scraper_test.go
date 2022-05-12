// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

// Test Scrape tests that the scraper assigns the metrics correctly
func TestScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	scraper := newIisReceiver(
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	scraper.newWatcher = newMockWatcherFactory(nil, 1)

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
}

func TestScrapeFailure(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	core, obs := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	rcvrSettings := componenttest.NewNopReceiverCreateSettings()
	rcvrSettings.Logger = logger

	scraper := newIisReceiver(
		rcvrSettings,
		cfg,
		consumertest.NewNop(),
	)

	expectedError := "failure to collect metric"
	mockWatcher, err := newMockWatcherFactory(fmt.Errorf(expectedError), 1)("", "", "")
	require.NoError(t, err)
	scraper.watcherRecorders = []watcherRecorder{
		{
			watcher: mockWatcher,
			recorder: func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisUptimeDataPoint(ts, int64(val))
			},
		},
	}

	scraper.scrape(context.Background())

	require.Equal(t, 1, obs.Len())
	log := obs.All()[0]
	require.Equal(t, log.Level, zapcore.WarnLevel)
	require.Equal(t, "error", log.Context[0].Key)
	require.EqualError(t, log.Context[0].Interface.(error), expectedError)
}

type mockPerfCounter struct {
	watchErr error
	value    float64
	winperfcounters.MetricRep
}

func newMockWatcherFactory(watchErr error, value float64) func(string, string,
	string) (winperfcounters.PerfCounterWatcher, error) {
	return func(string, string, string) (winperfcounters.PerfCounterWatcher, error) {
		return &mockPerfCounter{watchErr: watchErr, value: value}, nil
	}
}

// Path
func (mpc *mockPerfCounter) Path() string {
	return ""
}

// ScrapeData
func (mpc *mockPerfCounter) ScrapeData() ([]winperfcounters.CounterValue, error) {
	return []winperfcounters.CounterValue{{Value: 1}}, mpc.watchErr
}

// Close
func (mpc *mockPerfCounter) Close() error {
	return nil
}

func (mpc *mockPerfCounter) GetMetricRep() winperfcounters.MetricRep {
	return winperfcounters.MetricRep{}
}
