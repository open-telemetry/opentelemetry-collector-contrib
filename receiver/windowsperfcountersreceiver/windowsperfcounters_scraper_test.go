// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
)

type mockPerfCounter struct {
	counterValues []winperfcounters.CounterValue
	metricRep     MetricRep
	path          string
	scrapeErr     error
	closeErr      error
}

func (w *mockPerfCounter) Path() string {
	return w.path
}

func (w *mockPerfCounter) ScrapeData() ([]winperfcounters.CounterValue, error) {
	return w.counterValues, w.scrapeErr
}

func (w *mockPerfCounter) Close() error {
	return w.closeErr
}

func mockPerfCounterFactory(mpc mockPerfCounter) newWatcherFunc {
	return func(string, string, string) (winperfcounters.PerfCounterWatcher, error) {
		return &mpc, nil
	}
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
				PerfCounters: []ObjectConfig{
					{Object: "Memory", Counters: []CounterConfig{{Name: "Committed Bytes", MetricRep: MetricRep{Name: "bytes.committed"}}}},
					{Object: "Processor", Instances: []string{"*"}, Counters: []CounterConfig{{Name: "% Idle Time", MetricRep: MetricRep{Name: "cpu.idle"}}}},
					{Object: "Processor", Instances: []string{"1", "2"}, Counters: []CounterConfig{{Name: "% Processor Time", MetricRep: MetricRep{Name: "processor.time"}}}},
				},
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: time.Minute, InitialDelay: time.Second},
			},
			expectedMetricPath: filepath.Join("testdata", "scraper", "standard.yaml"),
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
				PerfCounters: []ObjectConfig{
					{Object: "Memory", Counters: []CounterConfig{{Name: "Committed Bytes", MetricRep: MetricRep{Name: "bytes.committed"}}}},
				},
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: time.Minute, InitialDelay: time.Second},
			},
			expectedMetricPath: filepath.Join("testdata", "scraper", "sum_metric.yaml"),
		},
		{
			name: "NoMetricDefinition",
			cfg: &Config{
				PerfCounters: []ObjectConfig{
					{Object: "Memory", Counters: []CounterConfig{{Name: "Committed Bytes"}}},
				},
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: time.Minute, InitialDelay: time.Second},
			},
			expectedMetricPath: filepath.Join("testdata", "scraper", "no_metric_def.yaml"),
		},
		{
			name: "InvalidCounter",
			cfg: &Config{
				PerfCounters: []ObjectConfig{
					{
						Object:   "Memory",
						Counters: []CounterConfig{{Name: "Committed Bytes", MetricRep: MetricRep{Name: "Committed Bytes"}}},
					},
					{
						Object:   "Invalid Object",
						Counters: []CounterConfig{{Name: "Invalid Counter", MetricRep: MetricRep{Name: "invalid"}}},
					},
				},
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: time.Minute, InitialDelay: time.Second},
			},
			startMessage: "some performance counters could not be initialized",
			startErr:     "failed to create perf counter with path \\Invalid Object\\Invalid Counter: The specified object was not found on the computer.\r\n",
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
			pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreMetricValues())
			require.NoError(t, err)
		})
	}
}

func TestInitWatchers(t *testing.T) {
	testCases := []struct {
		name          string
		cfgs          []ObjectConfig
		expectedErr   string
		expectedPaths []string
	}{
		{
			name: "basicPath",
			cfgs: []ObjectConfig{
				{
					Object:   "Memory",
					Counters: []CounterConfig{{Name: "Committed Bytes"}},
				},
			},
			expectedPaths: []string{"\\Memory\\Committed Bytes"},
		},
		{
			name: "multiplePaths",
			cfgs: []ObjectConfig{
				{
					Object:   "Memory",
					Counters: []CounterConfig{{Name: "Committed Bytes"}},
				},
				{
					Object:   "Memory",
					Counters: []CounterConfig{{Name: "Available Bytes"}},
				},
			},
			expectedPaths: []string{"\\Memory\\Committed Bytes", "\\Memory\\Available Bytes"},
		},
		{
			name: "multipleIndividualCounters",
			cfgs: []ObjectConfig{
				{
					Object: "Memory",
					Counters: []CounterConfig{
						{Name: "Committed Bytes"},
						{Name: "Available Bytes"},
					},
				},
				{
					Object:   "Memory",
					Counters: []CounterConfig{},
				},
			},
			expectedPaths: []string{"\\Memory\\Committed Bytes", "\\Memory\\Available Bytes"},
		},
		{
			name: "invalidCounter",
			cfgs: []ObjectConfig{
				{
					Object:   "Broken",
					Counters: []CounterConfig{{Name: "Broken Counter"}},
				},
			},

			expectedErr: "failed to create perf counter with path \\Broken\\Broken Counter: The specified object was not found on the computer.\r\n",
		},
		{
			name: "multipleInvalidCounters",
			cfgs: []ObjectConfig{
				{
					Object:   "Broken",
					Counters: []CounterConfig{{Name: "Broken Counter"}},
				},
				{
					Object:   "Broken part 2",
					Counters: []CounterConfig{{Name: "Broken again"}},
				},
			},
			expectedErr: "failed to create perf counter with path \\Broken\\Broken Counter: The specified object was not found on the computer.\r\n; failed to create perf counter with path \\Broken part 2\\Broken again: The specified object was not found on the computer.\r\n",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			s := &scraper{cfg: &Config{PerfCounters: test.cfgs}, newWatcher: winperfcounters.NewWatcher}
			watchers, errs := s.initWatchers()
			if test.expectedErr != "" {
				require.EqualError(t, errs, test.expectedErr)
			} else {
				require.NoError(t, errs)
			}
			for i, watcher := range watchers {
				require.Equal(t, test.expectedPaths[i], watcher.Path())
			}
		})
	}
}

func TestScrape(t *testing.T) {
	testCases := []struct {
		name              string
		cfg               Config
		mockCounterValues []winperfcounters.CounterValue
	}{
		{
			name: "metricsWithoutInstance",
			cfg: Config{
				PerfCounters: []ObjectConfig{
					{
						Counters: []CounterConfig{
							{
								MetricRep: MetricRep{
									Name: "metric1",
								},
							},
							{
								MetricRep: MetricRep{
									Name: "metric2",
									Attributes: map[string]string{
										"test.attribute": "test-value",
									},
								},
							},
						},
					},
				},
				MetricMetaData: map[string]MetricConfig{
					"metric1": {Description: "metric1 description", Unit: "1"},
					"metric2": {Description: "metric2 description", Unit: "2"},
				},
			},
			mockCounterValues: []winperfcounters.CounterValue{{Value: 1.0}},
		},
		{
			name: "metricsWithInstance",
			cfg: Config{
				PerfCounters: []ObjectConfig{
					{
						Counters: []CounterConfig{
							{
								MetricRep: MetricRep{
									Name: "metric1",
								},
							},
							{
								MetricRep: MetricRep{
									Name: "metric2",
									Attributes: map[string]string{
										"test.attribute": "test-value",
									},
								},
							},
						},
					},
				},
				MetricMetaData: map[string]MetricConfig{
					"metric1": {Description: "metric1 description", Unit: "1"},
					"metric2": {Description: "metric2 description", Unit: "2"},
				},
			},
			mockCounterValues: []winperfcounters.CounterValue{{InstanceName: "Test Instance", Value: 1.0}},
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			mpc := mockPerfCounter{counterValues: test.mockCounterValues}
			s := &scraper{cfg: &test.cfg, newWatcher: mockPerfCounterFactory(mpc)}
			errs := s.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, errs)

			m, err := s.scrape(context.Background())
			require.NoError(t, err)
			require.Equal(t, 1, m.ResourceMetrics().Len())
			require.Equal(t, 1, m.ResourceMetrics().At(0).ScopeMetrics().Len())

			metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			metrics.Sort(func(a, b pmetric.Metric) bool {
				return a.Name() < b.Name()
			})
			curMetricsNum := 0
			for _, pc := range test.cfg.PerfCounters {
				for _, counterCfg := range pc.Counters {
					metric := metrics.At(curMetricsNum)
					assert.Equal(t, counterCfg.MetricRep.Name, metric.Name())
					metricData := test.cfg.MetricMetaData[counterCfg.MetricRep.Name]
					assert.Equal(t, metricData.Description, metric.Description())
					assert.Equal(t, metricData.Unit, metric.Unit())
					dps := metric.Gauge().DataPoints()
					assert.Equal(t, len(test.mockCounterValues), dps.Len())
					for dpIdx, val := range test.mockCounterValues {
						assert.Equal(t, val.Value, dps.At(dpIdx).DoubleValue())
						expectedAttributeLen := len(counterCfg.MetricRep.Attributes)
						if val.InstanceName != "" {
							expectedAttributeLen++
						}
						assert.Equal(t, expectedAttributeLen, dps.At(dpIdx).Attributes().Len())
						dps.At(dpIdx).Attributes().Range(func(k string, v pcommon.Value) bool {
							if k == instanceLabelName {
								assert.Equal(t, val.InstanceName, v.Str())
								return true
							}
							assert.Equal(t, counterCfg.MetricRep.Attributes[k], v.Str())
							return true
						})
					}
					curMetricsNum++
				}
			}
		})
	}
}
