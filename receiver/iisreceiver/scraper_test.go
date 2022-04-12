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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

// Test Scrape tests that the scraper assigns the metrics correctly
func TestScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	scraper := newIisReceiver(
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)

	scraper.watchers = allFakePerfCounters()

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.json")
	golden.WriteMetrics(expectedFile, actualMetrics)
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(actualMetrics, expectedMetrics))
}

func TestScrapeFailure(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	scraper := newIisReceiver(
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)

	expectedError := "failure to collect metric"
	scraper.watchers = []winperfcounters.PerfCounterWatcher{
		newMockPerfCounter(fmt.Errorf(expectedError), 1, winperfcounters.MetricRep{Name: "iis.uptime"}),
	}

	_, err := scraper.scrape(context.Background())
	require.EqualError(t, err, expectedError)
}

type mockPerfCounter struct {
	watchErr error
	value    float64
	winperfcounters.MetricRep
}

func allFakePerfCounters() []winperfcounters.PerfCounterWatcher {
	return []winperfcounters.PerfCounterWatcher{
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.connection.active"}),
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.connection.anonymous"}),
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.connection.attempt.count"}),
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.network.blocked"}),
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.network.file.count", Attributes: map[string]string{metadata.A.Direction: "received"}}),
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.network.io", Attributes: map[string]string{metadata.A.Direction: "sent"}}),
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.request.count", Attributes: map[string]string{metadata.A.Request: "get"}}),
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.request.queue.age.max"}),
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.request.queue.count"}),
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.request.rejected"}),
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.thread.active"}),
		newMockPerfCounter(nil, 1, winperfcounters.MetricRep{Name: "iis.uptime"}),
	}
}

func newMockPerfCounter(watchErr error, value float64, metric winperfcounters.MetricRep) *mockPerfCounter {
	return &mockPerfCounter{watchErr: watchErr, value: value, MetricRep: metric}
}

// Path
func (mpc *mockPerfCounter) Path() string {
	return ""
}

// ScrapeData
func (mpc *mockPerfCounter) ScrapeData() (winperfcounters.CounterValue, error) {
	return winperfcounters.CounterValue{Value: 1, MetricRep: mpc.MetricRep}, mpc.watchErr
}

// Close
func (mpc *mockPerfCounter) Close() error {
	return nil
}

func (mpc *mockPerfCounter) GetMetricRep() winperfcounters.MetricRep {
	return mpc.MetricRep
}
