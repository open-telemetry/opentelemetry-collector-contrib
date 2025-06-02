// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

// Test Scrape tests that the scraper assigns the metrics correctly
func TestScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	scraper := newIisReceiver(
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	scraper.newWatcher = newMockWatcherFactory(nil)
	scraper.newWatcherFromPath = newMockWatcherFactorFromPath(nil, 1)
	scraper.expandWildcardPath = func(s string) ([]string, error) {
		return []string{strings.Replace(s, "*", "Instance", 1)}, nil
	}

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScrapeFailure(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	core, obs := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	rcvrSettings := receivertest.NewNopSettings(metadata.Type)
	rcvrSettings.Logger = logger

	scraper := newIisReceiver(
		rcvrSettings,
		cfg,
		consumertest.NewNop(),
	)

	expectedError := "failure to collect metric"
	mockWatcher, err := newMockWatcherFactory(errors.New(expectedError))("", "", "")
	require.NoError(t, err)
	scraper.totalWatcherRecorders = []watcherRecorder{
		{
			watcher: mockWatcher,
			recorder: func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisUptimeDataPoint(ts, int64(val))
			},
		},
	}

	_, err = scraper.scrape(context.Background())
	require.NoError(t, err)

	require.Equal(t, 1, obs.Len())
	log := obs.All()[0]
	require.Equal(t, zapcore.WarnLevel, log.Level)
	require.Equal(t, "error", log.Context[0].Key)
	require.EqualError(t, log.Context[0].Interface.(error), expectedError)
}

func TestMaxQueueItemAgeScrapeFailure(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	core, obs := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	rcvrSettings := receivertest.NewNopSettings(metadata.Type)
	rcvrSettings.Logger = logger

	scraper := newIisReceiver(
		rcvrSettings,
		cfg,
		consumertest.NewNop(),
	)

	expectedError := "failure to collect metric"
	mockWatcher, err := newMockWatcherFactory(errors.New(expectedError))("", "", "")
	require.NoError(t, err)
	scraper.queueMaxAgeWatchers = []instanceWatcher{
		{
			watcher:  mockWatcher,
			instance: "Instance",
		},
	}

	_, err = scraper.scrape(context.Background())
	require.NoError(t, err)

	require.Equal(t, 1, obs.Len())
	log := obs.All()[0]
	require.Equal(t, zapcore.WarnLevel, log.Level)
	require.Equal(t, "error", log.Context[0].Key)
	require.EqualError(t, log.Context[0].Interface.(error), expectedError)
}

func TestMaxQueueItemAgeNegativeDenominatorScrapeFailure(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	rcvrSettings := receivertest.NewNopSettings(metadata.Type)

	scraper := newIisReceiver(
		rcvrSettings,
		cfg,
		consumertest.NewNop(),
	)

	expectedError := "Failed to scrape counter \"counter\": A counter with a negative denominator value was detected.\r\n"
	mockWatcher, err := newMockWatcherFactory(errors.New(expectedError))("", "", "")
	require.NoError(t, err)
	scraper.queueMaxAgeWatchers = []instanceWatcher{
		{
			watcher:  mockWatcher,
			instance: "Instance",
		},
	}

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected_negative_denominator.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

type mockPerfCounter struct {
	watchErr error
	value    float64
}

func newMockWatcherFactory(watchErr error) func(string, string,
	string) (winperfcounters.PerfCounterWatcher, error) {
	return func(string, string, string) (winperfcounters.PerfCounterWatcher, error) {
		return &mockPerfCounter{watchErr: watchErr, value: 1}, nil
	}
}

func newMockWatcherFactorFromPath(watchErr error, value float64) func(string) (winperfcounters.PerfCounterWatcher, error) {
	return func(_ string) (winperfcounters.PerfCounterWatcher, error) {
		return &mockPerfCounter{watchErr: watchErr, value: value}, nil
	}
}

// ScrapeRawValue implements winperfcounters.PerfCounterWatcher.
func (mpc *mockPerfCounter) ScrapeRawValue(_ *int64) (bool, error) {
	panic("unimplemented")
}

// ScrapeRawValues implements winperfcounters.PerfCounterWatcher.
func (mpc *mockPerfCounter) ScrapeRawValues() ([]winperfcounters.RawCounterValue, error) {
	panic("unimplemented")
}

// Path
func (mpc *mockPerfCounter) Path() string {
	return ""
}

// ScrapeData
func (mpc *mockPerfCounter) ScrapeData() ([]winperfcounters.CounterValue, error) {
	return []winperfcounters.CounterValue{{InstanceName: "Instance", Value: 1}}, mpc.watchErr
}

// Close
func (mpc *mockPerfCounter) Close() error {
	return nil
}

func (mpc *mockPerfCounter) Reset() error {
	return nil
}
