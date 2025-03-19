// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowsperfcountersreceiver

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
)

type mockPerfCounter struct {
	counterValues []winperfcounters.CounterValue
	path          string
	scrapeErr     error
	closeErr      error
	resetErr      error
}

func (w *mockPerfCounter) Reset() error {
	return w.resetErr
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

func mockPerfCounterFactoryInvocations(mpcs ...mockPerfCounter) newWatcherFunc {
	invocationNum := 0

	return func(string, string, string) (winperfcounters.PerfCounterWatcher, error) {
		if invocationNum == len(mpcs) {
			return nil, fmt.Errorf("invoked watcher %d times but only %d were setup", invocationNum+1, len(mpcs))
		}
		mpc := mpcs[invocationNum]
		invocationNum++

		return &mpc, nil
	}
}

func Test_WindowsPerfCounterScraper(t *testing.T) {
	type testCase struct {
		name string
		cfg  *Config

		startMessage string
		startErr     string

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
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: time.Minute, InitialDelay: time.Second},
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
						Sum:         SumMetric{Aggregation: "cumulative", Monotonic: true},
					},
				},
				PerfCounters: []ObjectConfig{
					{Object: "Memory", Counters: []CounterConfig{{Name: "Committed Bytes", MetricRep: MetricRep{Name: "bytes.committed"}}}},
				},
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: time.Minute, InitialDelay: time.Second},
			},
			expectedMetricPath: filepath.Join("testdata", "scraper", "sum_metric.yaml"),
		},
		{
			name: "NoMetricDefinition",
			cfg: &Config{
				PerfCounters: []ObjectConfig{
					{Object: "Memory", Counters: []CounterConfig{{Name: "Committed Bytes"}}},
				},
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: time.Minute, InitialDelay: time.Second},
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
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: time.Minute, InitialDelay: time.Second},
			},
			startMessage: "some performance counters could not be initialized",
			startErr:     "failed to create perf counter with path \\Invalid Object\\Invalid Counter: The specified object was not found on the computer.\r\n",
		},
		{
			name: "NoMatchingInstance",
			cfg: &Config{
				MetricMetaData: map[string]MetricConfig{
					"no.matching.instance": {
						Unit:  "%",
						Gauge: GaugeMetric{},
					},
				},
				PerfCounters: []ObjectConfig{
					{
						Object:    ".NET CLR Memory",
						Instances: []string{"NoMatchingInstance*"},
						Counters:  []CounterConfig{{Name: "% Time in GC", MetricRep: MetricRep{Name: "no.matching.instance"}}},
					},
				},
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: time.Minute, InitialDelay: time.Second},
			},
			expectedMetricPath: filepath.Join("testdata", "scraper", "no_matching_instance.yaml"),
		},
		{
			name: "MetricDefinedButNoScrapedValue",
			cfg: &Config{
				MetricMetaData: map[string]MetricConfig{
					"cpu.idle": {
						Description: "percentage of time CPU is idle.",
						Unit:        "%",
						Gauge:       GaugeMetric{},
					},
					"no.counter": {
						Description: "there is no counter or data for this metric",
						Unit:        "By",
						Gauge:       GaugeMetric{},
					},
				},
				PerfCounters: []ObjectConfig{
					{Object: "Processor", Instances: []string{"_Total"}, Counters: []CounterConfig{{Name: "% Idle Time", MetricRep: MetricRep{Name: "cpu.idle"}}}},
				},
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: time.Minute, InitialDelay: time.Second},
			},
			expectedMetricPath: filepath.Join("testdata", "scraper", "metric_not_scraped.yaml"),
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
				assert.Equal(t, zapcore.WarnLevel, log.Level)
				assert.Equal(t, test.startMessage, log.Message)
				assert.Equal(t, "error", log.Context[0].Key)
				assert.EqualError(t, log.Context[0].Interface.(error), test.startErr)
				return
			}
			require.Equal(t, 0, obs.Len())
			require.NoError(t, err)

			actualMetrics, err := scraper.scrape(context.Background())
			require.NoError(t, err)

			err = scraper.shutdown(context.Background())

			require.NoError(t, err)
			expectedMetrics, err := golden.ReadMetrics(test.expectedMetricPath)
			require.NoError(t, err)

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
				// Scraping test host means static values, timestamps and instance counts are unreliable. ScopeMetrics order is also unpredictable.
				// The check only takes the first instance of multi-instance counters and assumes that the other instances would be included.
				pmetrictest.IgnoreSubsequentDataPoints("cpu.idle"),
				pmetrictest.IgnoreSubsequentDataPoints("processor.time"),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreScopeMetricsOrder(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricValues(),
				pmetrictest.IgnoreTimestamp(),
			))
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
			s := &windowsPerfCountersScraper{cfg: &Config{PerfCounters: test.cfgs}, newWatcher: winperfcounters.NewWatcher}
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

func TestWatcherResetFailure(t *testing.T) {
	const errMsg string = "failed to reset watcher"
	mpc := mockPerfCounter{
		counterValues: []winperfcounters.CounterValue{{Value: 1.0}},
		resetErr:      errors.New(errMsg),
	}

	cfg := Config{
		PerfCounters: []ObjectConfig{
			{
				Counters: []CounterConfig{
					{
						MetricRep: MetricRep{
							Name: "metric",
						},
						RecreateQuery: true,
					},
				},
			},
		},
		MetricMetaData: map[string]MetricConfig{
			"metric": {Description: "description", Unit: "1"},
		},
	}

	core, _ := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger = logger

	s := &windowsPerfCountersScraper{cfg: &cfg, settings: settings, newWatcher: mockPerfCounterFactoryInvocations(mpc)}
	errs := s.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, errs)

	vals, err := s.scrape(context.Background())

	if assert.Error(t, err) {
		assert.Equal(t, errMsg, err.Error())
	}
	assert.NotEmpty(t, vals) // Still attempts scraping using previous query
}

func TestScrape(t *testing.T) {
	testCases := []struct {
		name             string
		cfg              Config
		mockPerfCounters []mockPerfCounter
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
			mockPerfCounters: []mockPerfCounter{
				{counterValues: []winperfcounters.CounterValue{{Value: 1.0}}},
				{counterValues: []winperfcounters.CounterValue{{Value: 2.0}}},
			},
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
			mockPerfCounters: []mockPerfCounter{
				{counterValues: []winperfcounters.CounterValue{{InstanceName: "Test Instance", Value: 1.0}}},
				{counterValues: []winperfcounters.CounterValue{{InstanceName: "Test Instance", Value: 2.0}}},
			},
		},
		{
			name: "metricsWithSingleCounterFailure",
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
							{
								MetricRep: MetricRep{
									Name: "metric3",
								},
							},
						},
					},
				},
				MetricMetaData: map[string]MetricConfig{
					"metric1": {Description: "metric1 description", Unit: "1"},
					"metric2": {Description: "metric2 description", Unit: "2"},
					"metric3": {Description: "metric3 description", Unit: "3"},
				},
			},
			mockPerfCounters: []mockPerfCounter{
				{counterValues: []winperfcounters.CounterValue{{InstanceName: "Test Instance", Value: 1.0}}},
				{scrapeErr: errors.New("unable to scrape metric 2")},
				{scrapeErr: errors.New("unable to scrape metric 3")},
			},
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			mpcs := test.mockPerfCounters
			testConfig := test.cfg
			s := &windowsPerfCountersScraper{cfg: &testConfig, newWatcher: mockPerfCounterFactoryInvocations(mpcs...)}
			errs := s.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, errs)

			var expectedErrors []error
			for _, mpc := range test.mockPerfCounters {
				if mpc.scrapeErr != nil {
					expectedErrors = append(expectedErrors, mpc.scrapeErr)
				}
			}

			m, err := s.scrape(context.Background())
			if len(expectedErrors) != 0 {
				var partialErr scrapererror.PartialScrapeError
				require.ErrorAs(t, err, &partialErr)
				require.Equal(t, len(expectedErrors), partialErr.Failed)
				expectedError := multierr.Combine(expectedErrors...)
				require.Equal(t, expectedError.Error(), partialErr.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, 1, m.ResourceMetrics().Len())
			require.Equal(t, 1, m.ResourceMetrics().At(0).ScopeMetrics().Len())

			metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			metrics.Sort(func(a, b pmetric.Metric) bool {
				return a.Name() < b.Name()
			})

			assert.Equal(t, len(test.mockPerfCounters)-len(expectedErrors), metrics.Len())

			curMetricsNum := 0
			for _, pc := range test.cfg.PerfCounters {
				for counterIdx, counterCfg := range pc.Counters {
					counterValues := test.mockPerfCounters[counterIdx].counterValues
					scrapeErr := test.mockPerfCounters[counterIdx].scrapeErr

					if scrapeErr != nil {
						require.Empty(t, counterValues, "Invalid test case. Scrape error and counter values simultaneously.")
						continue // no data for this counter.
					}

					metric := metrics.At(curMetricsNum)
					assert.Equal(t, counterCfg.MetricRep.Name, metric.Name())
					metricData := test.cfg.MetricMetaData[counterCfg.MetricRep.Name]
					assert.Equal(t, metricData.Description, metric.Description())
					assert.Equal(t, metricData.Unit, metric.Unit())
					dps := metric.Gauge().DataPoints()

					assert.Equal(t, len(counterValues), dps.Len())
					for dpIdx, val := range counterValues {
						assert.Equal(t, val.Value, dps.At(dpIdx).DoubleValue())
						expectedAttributeLen := len(counterCfg.MetricRep.Attributes)
						if val.InstanceName != "" {
							expectedAttributeLen++
						}
						assert.Equal(t, expectedAttributeLen, dps.At(dpIdx).Attributes().Len())
						for k, v := range dps.At(dpIdx).Attributes().All() {
							if k == instanceLabelName {
								assert.Equal(t, val.InstanceName, v.Str())
								continue
							}
							assert.Equal(t, counterCfg.MetricRep.Attributes[k], v.Str())
						}
					}
					curMetricsNum++
				}
			}
		})
	}
}
