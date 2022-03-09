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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver/internal/third_party/telegraf/win_perf_counters"
)

type mockPerfCounter struct {
	path        string
	scrapeErr   error
	shutdownErr error
}

func newMockPerfCounter(path string, scrapeErr, shutdownErr error) *mockPerfCounter {
	return &mockPerfCounter{path: path, scrapeErr: scrapeErr, shutdownErr: shutdownErr}
}

// Path
func (mpc *mockPerfCounter) Path() string {
	return mpc.path
}

// ScrapeData
func (mpc *mockPerfCounter) ScrapeData() ([]win_perf_counters.CounterValue, error) {
	return []win_perf_counters.CounterValue{{Value: 0}}, mpc.scrapeErr
}

// Close
func (mpc *mockPerfCounter) Close() error {
	return mpc.shutdownErr
}

func Test_WindowsPerfCounterScraper(t *testing.T) {
	type expectedMetric struct {
		name                string
		instanceLabelValues []string
	}

	type testCase struct {
		name string
		cfg  *Config

		newErr          string
		mockCounterPath string
		startMessage    string
		startErr        string
		scrapeErr       error
		shutdownErr     error

		expectedMetrics []expectedMetric
	}

	defaultConfig := &Config{
		PerfCounters: []PerfCounterConfig{
			{
				Object:   "object",
				Counters: []CounterConfig{{CounterName: "Committed Bytes", MetricName: "commited.bytes"}},
			},
		},
		MetricMetaData: []MetricConfig{
			{
				MetricName:  "metric",
				Description: "desc",
				Unit:        "1",
				Gauge: GaugeMetric{
					ValueType: "double",
				},
			},
		},
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: time.Minute},
	}

	testCases := []testCase{
		{
			name: "Standard",
			cfg: &Config{
				PerfCounters: []PerfCounterConfig{
					{Object: "Memory", Counters: []CounterConfig{{CounterName: "Committed Bytes", MetricName: "Committed Bytes"}}},
					{Object: "Processor", Instances: []string{"*"}, Counters: []CounterConfig{{CounterName: "% Idle Time", MetricName: "cpu.idle"}}},
					{Object: "Processor", Instances: []string{"1", "2"}, Counters: []CounterConfig{{CounterName: "% Processor Time", MetricName: "bytes.commited"}}},
				},
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: time.Minute},
			},
			expectedMetrics: []expectedMetric{
				{name: `\Memory\Committed Bytes`},
				{name: `cpu.idle`, instanceLabelValues: []string{"*"}},
				{name: `bytes.commited"`, instanceLabelValues: []string{"1"}},
				{name: `bytes.commited"`, instanceLabelValues: []string{"2"}},
			},
		},
		{
			name: "InvalidCounter",
			cfg: &Config{
				PerfCounters: []PerfCounterConfig{
					{
						Object:   "Memory",
						Counters: []CounterConfig{{CounterName: "Committed Bytes", MetricName: "Committed Bytes"}},
					},
					{
						Object:   "Invalid Object",
						Counters: []CounterConfig{{CounterName: "Invalid Counter", MetricName: "invalid"}},
					},
				},
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: time.Minute},
			},
			startMessage: "some performance counters could not be initialized",
			startErr:     "counter \\Invalid Object\\Invalid Counter: The specified object was not found on the computer.\r\n",
		},
		{
			name:      "ScrapeError",
			scrapeErr: errors.New("err1"),
		},
		{
			name:            "CloseError",
			expectedMetrics: []expectedMetric{{name: ""}},
			shutdownErr:     errors.New("err1"),
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
			scraper, err := newScraper(cfg, logger)
			if test.newErr != "" {
				require.EqualError(t, err, test.newErr)
				return
			}

			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)
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

			if test.mockCounterPath != "" || test.scrapeErr != nil || test.shutdownErr != nil {
				for i := range scraper.counters {
					scraper.counters[i].CounterScraper = newMockPerfCounter(test.mockCounterPath, test.scrapeErr, test.shutdownErr)
				}
			}

			md, err := scraper.scrape(context.Background())
			if test.scrapeErr != nil {
				assert.Equal(t, err, test.scrapeErr)
			} else {
				require.NoError(t, err)
			}
			metrics := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
			require.Equal(t, len(test.expectedMetrics), metrics.Len())
			for i, e := range test.expectedMetrics {
				metric := metrics.At(i)
				assert.Equal(t, e.name, metric.Name())

				ddp := metric.Gauge().DataPoints()

				var allInstances bool
				for _, v := range e.instanceLabelValues {
					if v == "*" {
						allInstances = true
						break
					}
				}

				if allInstances {
					require.GreaterOrEqual(t, ddp.Len(), 1)
				} else {
					expectedDataPoints := 1
					if len(e.instanceLabelValues) > 0 {
						expectedDataPoints = len(e.instanceLabelValues)
					}

					require.Equal(t, expectedDataPoints, ddp.Len())
				}

				if len(e.instanceLabelValues) > 0 {
					instanceLabelValues := make([]string, 0, ddp.Len())
					for i := 0; i < ddp.Len(); i++ {
						instanceLabelValue, ok := ddp.At(i).Attributes().Get(instanceLabelName)
						require.Truef(t, ok, "data point was missing %q label", instanceLabelName)
						instanceLabelValues = append(instanceLabelValues, instanceLabelValue.StringVal())
					}

					if !allInstances {
						for _, v := range e.instanceLabelValues {
							assert.Contains(t, instanceLabelValues, v)
						}
					}
				}
			}

			err = scraper.shutdown(context.Background())
			if test.shutdownErr != nil {
				assert.Equal(t, err, test.shutdownErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
