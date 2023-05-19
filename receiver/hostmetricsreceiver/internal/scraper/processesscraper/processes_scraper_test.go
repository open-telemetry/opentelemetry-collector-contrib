// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processesscraper

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper/internal/metadata"
)

var (
	expectProcessesCountMetric   = runtime.GOOS == "linux" || runtime.GOOS == "openbsd" || runtime.GOOS == "darwin" || runtime.GOOS == "freebsd" || runtime.GOOS == "solaris"
	expectProcessesCreatedMetric = runtime.GOOS == "linux" || runtime.GOOS == "openbsd"
)

func TestScrape(t *testing.T) {
	type testCase struct {
		name         string
		getMiscStats func() (*load.MiscStat, error)
		getProcesses func() ([]proc, error)
		expectedErr  string
		validate     func(*testing.T, pmetric.MetricSlice)
	}

	testCases := []testCase{{
		name:     "Standard",
		validate: validateRealData,
	}, {
		name:         "FakeData",
		getMiscStats: func() (*load.MiscStat, error) { return &fakeData, nil },
		getProcesses: func() ([]proc, error) { return fakeProcessesData, nil },
		validate:     validateFakeData,
	}, {
		name:         "ErrorFromMiscStat",
		getMiscStats: func() (*load.MiscStat, error) { return &load.MiscStat{}, errors.New("err1") },
		expectedErr:  "err1",
	}, {
		name:         "ErrorFromProcesses",
		getProcesses: func() ([]proc, error) { return nil, errors.New("err2") },
		expectedErr:  "err2",
	}, {
		name:         "ErrorFromProcessShouldBeIgnored",
		getProcesses: func() ([]proc, error) { return []proc{errProcess{}}, nil },
	}, {
		name:     "Validate Start Time",
		validate: validateStartTime,
	}}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper := newProcessesScraper(context.Background(), receivertest.NewNopCreateSettings(), &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			})
			err := scraper.start(context.Background(), componenttest.NewNopHost())
			assert.NoError(t, err, "Failed to initialize processes scraper: %v", err)

			// Override scraper methods if we are mocking out for this test case
			if test.getMiscStats != nil {
				scraper.getMiscStats = test.getMiscStats
			}
			if test.getProcesses != nil {
				scraper.getProcesses = test.getProcesses
			}

			md, err := scraper.scrape(context.Background())

			expectedMetricCount := 0
			if expectProcessesCountMetric {
				expectedMetricCount++
			}
			if expectProcessesCreatedMetric {
				expectedMetricCount++
			}

			if (expectProcessesCountMetric || expectProcessesCreatedMetric) && test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)

				isPartial := scrapererror.IsPartialScrapeError(err)
				assert.Truef(t, isPartial, "expected partial scrape error, have %+v", err)
				if isPartial {
					var scraperErr scrapererror.PartialScrapeError
					require.ErrorAs(t, err, &scraperErr)
					assert.Equal(t, expectedMetricCount, scraperErr.Failed)
				}

				return
			}

			if test.expectedErr == "" {
				assert.NoErrorf(t, err, "Failed to scrape metrics: %v", err)
			}

			assert.Equal(t, expectedMetricCount, md.MetricCount())

			if expectedMetricCount > 0 {
				metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				if test.validate != nil {
					test.validate(t, metrics)
				}

				internal.AssertSameTimeStampForAllMetrics(t, metrics)
			}
		})
	}
}

func validateRealData(t *testing.T, metrics pmetric.MetricSlice) {
	metricIndex := 0
	if expectProcessesCountMetric {
		countMetric := metrics.At(metricIndex)
		metricIndex++
		assert.Equal(t, "system.processes.count", countMetric.Name())

		assertContainsStatus := func(statusVal string) {
			points := countMetric.Sum().DataPoints()
			for i := 0; i < points.Len(); i++ {
				v, ok := points.At(i).Attributes().Get("status")
				if ok && v.Str() == statusVal {
					return
				}
			}
			assert.Failf(t, "missing-metric", "metric is missing %q status label", statusVal)
		}
		assertContainsStatus(metadata.AttributeStatusRunning.String())
		assertContainsStatus(metadata.AttributeStatusBlocked.String())
	}

	if expectProcessesCreatedMetric {
		createdMetric := metrics.At(metricIndex)
		assert.Equal(t, "system.processes.created", createdMetric.Name())
		createdMetric = metrics.At(1)
		assert.Equal(t, "system.processes.created", createdMetric.Name())
		assert.Equal(t, 1, createdMetric.Sum().DataPoints().Len())
		assert.Equal(t, 0, createdMetric.Sum().DataPoints().At(0).Attributes().Len())
	}
}

func validateStartTime(t *testing.T, metrics pmetric.MetricSlice) {
	startTime, err := host.BootTime()
	assert.NoError(t, err)
	for i := 0; i < metricsLength; i++ {
		internal.AssertSumMetricStartTimeEquals(t, metrics.At(i), pcommon.Timestamp(startTime*1e9))
	}
}

var fakeData = load.MiscStat{
	ProcsCreated: 1,
	ProcsRunning: 2,
	ProcsBlocked: 3,
	ProcsTotal:   30,
}

var fakeProcessesData = []proc{
	fakeProcess(process.Wait),
	fakeProcess(process.Blocked), fakeProcess(process.Blocked),
	fakeProcess(process.Running), fakeProcess(process.Running), fakeProcess(process.Running),
	fakeProcess(process.Sleep), fakeProcess(process.Sleep), fakeProcess(process.Sleep), fakeProcess(process.Sleep),
	fakeProcess(process.Stop), fakeProcess(process.Stop), fakeProcess(process.Stop), fakeProcess(process.Stop), fakeProcess(process.Stop),
	fakeProcess(process.Zombie), fakeProcess(process.Zombie), fakeProcess(process.Zombie), fakeProcess(process.Zombie), fakeProcess(process.Zombie), fakeProcess(process.Zombie),
}

type errProcess struct{}

func (e errProcess) Status() ([]string, error) {
	return []string{""}, errors.New("errProcess")
}

type fakeProcess string

func (f fakeProcess) Status() ([]string, error) {
	return []string{string(f)}, nil
}

func validateFakeData(t *testing.T, metrics pmetric.MetricSlice) {
	metricIndex := 0
	if expectProcessesCountMetric {
		countMetric := metrics.At(metricIndex)
		metricIndex++
		assert.Equal(t, "system.processes.count", countMetric.Name())

		points := countMetric.Sum().DataPoints()
		attrs := map[string]int64{}
		for i := 0; i < points.Len(); i++ {
			point := points.At(i)
			val, ok := point.Attributes().Get("status")
			assert.Truef(t, ok, "Missing status attribute in data point %d", i)
			attrs[val.Str()] = point.IntValue()
		}

		assert.Equal(t, attrs, map[string]int64{
			metadata.AttributeStatusBlocked.String():  3,
			metadata.AttributeStatusPaging.String():   1,
			metadata.AttributeStatusRunning.String():  2,
			metadata.AttributeStatusSleeping.String(): 4,
			metadata.AttributeStatusStopped.String():  5,
			metadata.AttributeStatusUnknown.String():  9,
			metadata.AttributeStatusZombies.String():  6,
		})
	}

	if expectProcessesCreatedMetric {
		createdMetric := metrics.At(metricIndex)
		assert.Equal(t, "system.processes.created", createdMetric.Name())
		assert.Equal(t, 1, createdMetric.Sum().DataPoints().Len())
		assert.Equal(t, 0, createdMetric.Sum().DataPoints().At(0).Attributes().Len())
	}
}
