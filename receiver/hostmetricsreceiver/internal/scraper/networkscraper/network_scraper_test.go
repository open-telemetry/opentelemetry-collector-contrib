// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package networkscraper

import (
	"context"
	"errors"
	"testing"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	type testCase struct {
		name                    string
		config                  *Config
		bootTimeFunc            func(context.Context) (uint64, error)
		ioCountersFunc          func(context.Context, bool) ([]net.IOCountersStat, error)
		connectionsFunc         func(context.Context, string) ([]net.ConnectionStat, error)
		conntrackFunc           func(context.Context) ([]net.FilterStat, error)
		expectConntrakMetrics   bool
		expectConnectionsMetric bool
		expectedStartTime       pcommon.Timestamp
		newErrRegex             string
		initializationErr       string
		expectedErr             string
		expectedErrCount        int
		mutateScraper           func(*scraper)
	}

	testCases := []testCase{
		{
			name: "Standard",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectConntrakMetrics:   true,
			expectConnectionsMetric: true,
		},
		{
			name: "Standard with direction removed",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectConntrakMetrics:   true,
			expectConnectionsMetric: true,
		},
		{
			name: "Validate Start Time",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			bootTimeFunc:            func(context.Context) (uint64, error) { return 100, nil },
			expectConntrakMetrics:   true,
			expectConnectionsMetric: true,
			expectedStartTime:       100 * 1e9,
		},
		{
			name: "Include Filter that matches nothing",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Include:              MatchConfig{filterset.Config{MatchType: "strict"}, []string{"@*^#&*$^#)"}},
			},
			expectConntrakMetrics:   false,
			expectConnectionsMetric: true,
		},
		{
			name: "Invalid Include Filter",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Include:              MatchConfig{Interfaces: []string{"test"}},
			},
			newErrRegex:             "^error creating network interface include filters:",
			expectConnectionsMetric: true,
		},
		{
			name: "Invalid Exclude Filter",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				Exclude:              MatchConfig{Interfaces: []string{"test"}},
			},
			newErrRegex:             "^error creating network interface exclude filters:",
			expectConnectionsMetric: true,
		},
		{
			name:                    "Boot Time Error",
			config:                  &Config{},
			bootTimeFunc:            func(context.Context) (uint64, error) { return 0, errors.New("err1") },
			initializationErr:       "err1",
			expectConnectionsMetric: true,
		},
		{
			name:                    "IOCounters Error",
			config:                  &Config{},
			ioCountersFunc:          func(context.Context, bool) ([]net.IOCountersStat, error) { return nil, errors.New("err2") },
			expectedErr:             "failed to read network IO stats: err2",
			expectedErrCount:        networkMetricsLen,
			expectConnectionsMetric: true,
		},
		{
			name:             "Connections Error",
			config:           &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()},
			connectionsFunc:  func(context.Context, string) ([]net.ConnectionStat, error) { return nil, errors.New("err3") },
			expectedErr:      "failed to read TCP connections: err3",
			expectedErrCount: connectionsMetricsLen,
		},
		{
			name: "Conntrack error ignored if metric disabled",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(), // conntrack metrics are disabled by default
			},
			conntrackFunc:           func(context.Context) ([]net.FilterStat, error) { return nil, errors.New("conntrack failed") },
			expectConntrakMetrics:   true,
			expectConnectionsMetric: true,
		},
		{
			name: "Connections metrics is disabled",
			config: func() *Config {
				cfg := Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()}
				cfg.MetricsBuilderConfig.Metrics.SystemNetworkConnections.Enabled = false
				return &cfg
			}(),
			connectionsFunc: func(context.Context, string) ([]net.ConnectionStat, error) {
				panic("should not be called")
			},
			expectConntrakMetrics: true,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper, err := newNetworkScraper(context.Background(), receivertest.NewNopSettings(), test.config)
			if test.mutateScraper != nil {
				test.mutateScraper(scraper)
			}
			if test.newErrRegex != "" {
				require.Error(t, err)
				require.Regexp(t, test.newErrRegex, err)
				return
			}
			require.NoError(t, err, "Failed to create network scraper: %v", err)

			if test.bootTimeFunc != nil {
				scraper.bootTime = test.bootTimeFunc
			}
			if test.ioCountersFunc != nil {
				scraper.ioCounters = test.ioCountersFunc
			}
			if test.connectionsFunc != nil {
				scraper.connections = test.connectionsFunc
			}
			if test.conntrackFunc != nil {
				scraper.conntrack = test.conntrackFunc
			}

			err = scraper.start(context.Background(), componenttest.NewNopHost())
			if test.initializationErr != "" {
				assert.EqualError(t, err, test.initializationErr)
				return
			}
			require.NoError(t, err, "Failed to initialize network scraper: %v", err)

			md, err := scraper.scrape(context.Background())
			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)

				isPartial := scrapererror.IsPartialScrapeError(err)
				assert.True(t, isPartial)
				if isPartial {
					var scraperErr scrapererror.PartialScrapeError
					require.ErrorAs(t, err, &scraperErr)
					assert.Equal(t, test.expectedErrCount, scraperErr.Failed)
				}

				return
			}
			require.NoError(t, err, "Failed to scrape metrics: %v", err)

			expectedMetricCount := 0
			if test.expectConntrakMetrics {
				expectedMetricCount += 4
			}
			if test.expectConnectionsMetric {
				expectedMetricCount++
			}
			assert.Equal(t, expectedMetricCount, md.MetricCount())

			metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			idx := 0
			if test.expectConnectionsMetric {
				assertNetworkConnectionsMetricValid(t, metrics.At(idx))
				idx++
			}
			if test.expectConntrakMetrics {
				assertNetworkIOMetricValid(t, metrics.At(idx), "system.network.dropped",
					test.expectedStartTime)
				assertNetworkIOMetricValid(t, metrics.At(idx+1), "system.network.errors", test.expectedStartTime)
				assertNetworkIOMetricValid(t, metrics.At(idx+2), "system.network.io", test.expectedStartTime)
				assertNetworkIOMetricValid(t, metrics.At(idx+3), "system.network.packets",
					test.expectedStartTime)
				internal.AssertSameTimeStampForMetrics(t, metrics, idx, idx+4)
			}
		})
	}
}

func assertNetworkIOMetricValid(t *testing.T, metric pmetric.Metric, expectedName string, startTime pcommon.Timestamp) {
	assert.Equal(t, expectedName, metric.Name())
	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, metric, startTime)
	}
	assert.GreaterOrEqual(t, metric.Sum().DataPoints().Len(), 2)
	internal.AssertSumMetricHasAttribute(t, metric, 0, "device")
	internal.AssertSumMetricHasAttribute(t, metric, 0, "direction")
}

func assertNetworkConnectionsMetricValid(t *testing.T, metric pmetric.Metric) {
	assert.Equal(t, "system.network.connections", metric.Name())
	internal.AssertSumMetricHasAttributeValue(t, metric, 0, "protocol",
		pcommon.NewValueStr(metadata.AttributeProtocolTcp.String()))
	internal.AssertSumMetricHasAttribute(t, metric, 0, "state")
	// Flaky test gives 12 or 13, so bound it
	assert.LessOrEqual(t, 12, metric.Sum().DataPoints().Len())
	assert.GreaterOrEqual(t, 13, metric.Sum().DataPoints().Len())
}
