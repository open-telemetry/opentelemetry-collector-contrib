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

package windowsperfcountersreceiver

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	windowsapi "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
)

type mockPerfCounter struct {
	path        string
	scrapeErr   error
	shutdownErr error
}

func Test_WindowsPerfCounterScraper(t *testing.T) {
	type testCase struct {
		name string
		cfg  *Config

		mockCounterPath string
		startMessage    string
		startErr        string

		expectedMetricPath string
	}

	defaultConfig := createDefaultConfig().(*Config)

	testCases := []testCase{
		{
			name: "Standard",
			cfg: &Config{
				MetricMetaData: map[string]MetricConfig{
					"cpu.idle": {
						Description: "percentage of time CPU is idle.",
						Unit:        "%",
						Gauge:       GaugeMetric{},
					},
					"bytes.committed": {
						Description: "number of bytes committed to memory",
						Unit:        "By",
						Gauge:       GaugeMetric{},
					},
					"processor.time": {
						Description: "amount of time processor is busy",
						Unit:        "%",
						Gauge:       GaugeMetric{},
					},
				},
				PerfCounters: []windowsapi.ObjectConfig{
					{Object: "Memory", Counters: []windowsapi.CounterConfig{{Name: "Committed Bytes", Metric: "bytes.committed"}}},
					{Object: "Processor", Instances: []string{"*"}, Counters: []windowsapi.CounterConfig{{Name: "% Idle Time", Metric: "cpu.idle"}}},
					{Object: "Processor", Instances: []string{"1", "2"}, Counters: []windowsapi.CounterConfig{{Name: "% Processor Time", Metric: "processor.time"}}},
				},
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: time.Minute},
			},
			expectedMetricPath: filepath.Join("testdata", "scraper", "standard.json"),
		},
		{
			name: "SumMetric",
			cfg: &Config{
				MetricMetaData: map[string]MetricConfig{
					"bytes.committed": {
						Description: "number of bytes committed to memory",
						Unit:        "By",
						Sum:         SumMetric{},
					},
				},
				PerfCounters: []windowsapi.ObjectConfig{
					{Object: "Memory", Counters: []windowsapi.CounterConfig{{Name: "Committed Bytes", Metric: "bytes.committed"}}},
				},
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: time.Minute},
			},
			expectedMetricPath: filepath.Join("testdata", "scraper", "sum_metric.json"),
		},
		{
			name: "NoMetricDefinition",
			cfg: &Config{
				PerfCounters: []windowsapi.ObjectConfig{
					{Object: "Memory", Counters: []windowsapi.CounterConfig{{Name: "Committed Bytes"}}},
				},
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: time.Minute},
			},
			expectedMetricPath: filepath.Join("testdata", "scraper", "no_metric_def.json"),
		},
		{
			name: "InvalidCounter",
			cfg: &Config{
				PerfCounters: []windowsapi.ObjectConfig{
					{
						Object:   "Memory",
						Counters: []windowsapi.CounterConfig{{Name: "Committed Bytes", Metric: "Committed Bytes"}},
					},
					{
						Object:   "Invalid Object",
						Counters: []windowsapi.CounterConfig{{Name: "Invalid Counter", Metric: "invalid"}},
					},
				},
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: time.Minute},
			},
			startMessage: "some performance counters could not be initialized",
			startErr:     "counter \\Invalid Object\\Invalid Counter: The specified object was not found on the computer.\r\n",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cfg := test.cfg
			if cfg == nil {
				cfg = defaultConfig
			}

			core, obs := observer.New(zapcore.WarnLevel)
			logger := zap.New(core)
			settings := componenttest.NewNopTelemetrySettings()
			settings.Logger = logger
			scraper := newScraper(cfg, settings)

			err := scraper.start(context.Background(), componenttest.NewNopHost())
			if test.startErr != "" {
				require.Equal(t, 1, obs.Len())
				log := obs.All()[0]
				assert.Equal(t, log.Level, zapcore.WarnLevel)
				assert.Equal(t, test.startMessage, log.Message)
				assert.Equal(t, "error", log.Context[0].Key)
				assert.EqualError(t, log.Context[0].Interface.(error), test.startErr)
				return
			}
			require.NoError(t, err)

			actualMetrics, err := scraper.scrape(context.Background())
			require.NoError(t, err)

			err = scraper.shutdown(context.Background())

			require.NoError(t, err)
			expectedMetrics, err := golden.ReadMetrics(test.expectedMetricPath)
			scrapertest.CompareMetrics(expectedMetrics, actualMetrics)
			require.NoError(t, err)
		})
	}
}
