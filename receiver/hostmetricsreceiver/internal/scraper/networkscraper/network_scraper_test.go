// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package networkscraper

import (
	"context"
	"errors"
	stdnet "net"
	"testing"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scrapertest"

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
		mutateScraper           func(*networkScraper)
	}

	testCases := []testCase{
		{
			name: "Standard",
			config: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
			},
			expectConntrakMetrics:   true,
			expectConnectionsMetric: true,
		},
		{
			name: "Standard with direction removed",
			config: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
			},
			expectConntrakMetrics:   true,
			expectConnectionsMetric: true,
		},
		{
			name: "Validate Start Time",
			config: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
			},
			bootTimeFunc:            func(context.Context) (uint64, error) { return 100, nil },
			expectConntrakMetrics:   true,
			expectConnectionsMetric: true,
			expectedStartTime:       100 * 1e9,
		},
		{
			name: "Include Filter that matches nothing",
			config: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				Include:              MatchConfig{filterset.Config{MatchType: "strict"}, []string{"@*^#&*$^#)"}},
			},
			expectConntrakMetrics:   false,
			expectConnectionsMetric: true,
		},
		{
			name: "Invalid Include Filter",
			config: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				Include:              MatchConfig{Interfaces: []string{"test"}},
			},
			newErrRegex:             "^error creating network interface include filters:",
			expectConnectionsMetric: true,
		},
		{
			name: "Invalid Exclude Filter",
			config: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
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
			config:           &Config{MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig()},
			connectionsFunc:  func(context.Context, string) ([]net.ConnectionStat, error) { return nil, errors.New("err3") },
			expectedErr:      "failed to read TCP connections: err3",
			expectedErrCount: 1,
		},
		{
			name: "Conntrack error ignored if metric disabled",
			config: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(), // conntrack metrics are disabled by default
			},
			conntrackFunc:           func(context.Context) ([]net.FilterStat, error) { return nil, errors.New("conntrack failed") },
			expectConntrakMetrics:   true,
			expectConnectionsMetric: true,
		},
		{
			name: "Connections metrics is disabled",
			config: func() *Config {
				cfg := Config{MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig()}
				cfg.Metrics.SystemNetworkConnections.Enabled = false
				cfg.Metrics.SystemNetworkConnectionCount.Enabled = false
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
			scraper, err := newNetworkScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), test.config)
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
			scraper.processName = func(context.Context, int32) (string, error) { return "processname", nil }
			scraper.interfaceAddrs = func() ([]stdnet.Addr, error) { return nil, nil }

			err = scraper.start(t.Context(), componenttest.NewNopHost())
			if test.initializationErr != "" {
				assert.EqualError(t, err, test.initializationErr)
				return
			}
			require.NoError(t, err, "Failed to initialize network scraper: %v", err)

			md, err := scraper.scrape(t.Context())
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

func TestScrapeNetworkConnectionCountMetric(t *testing.T) {
	cfg := Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		Connections: ConnectionConfig{
			IncludeProcesses:   ProcessMatchConfig{Config: filterset.Config{MatchType: "strict"}, Names: []string{"app"}},
			ExcludeProcesses:   ProcessMatchConfig{Config: filterset.Config{MatchType: "strict"}, Names: []string{"skip"}},
			IncludePorts:       []uint32{443, 8443},
			ExcludePorts:       []uint32{8443},
			ExcludeLocalhost:   true,
			ExcludeListenPorts: true,
		},
	}
	cfg.Metrics.SystemNetworkConnections.Enabled = false
	cfg.Metrics.SystemNetworkConnectionCount.Enabled = true

	md, processNameCalls := scrapeConnectionCountMetric(t, &cfg, []net.ConnectionStat{
		{Status: "ESTABLISHED", Pid: 1, Laddr: net.Addr{IP: "10.0.0.1", Port: 50001}, Raddr: net.Addr{IP: "203.0.113.10", Port: 443}},
		{Status: "ESTABLISHED", Pid: 1, Laddr: net.Addr{IP: "10.0.0.1", Port: 50002}, Raddr: net.Addr{IP: "203.0.113.10", Port: 443}},
		{Status: "SYN_SENT", Pid: 1, Laddr: net.Addr{IP: "10.0.0.1", Port: 50003}, Raddr: net.Addr{IP: "203.0.113.10", Port: 443}},
		{Status: "ESTABLISHED", Pid: 1, Laddr: net.Addr{IP: "10.0.0.1", Port: 50004}, Raddr: net.Addr{}},
		{Status: "ESTABLISHED", Pid: 1, Laddr: net.Addr{IP: "10.0.0.1", Port: 50005}, Raddr: net.Addr{IP: "203.0.113.10"}},
		{Status: "ESTABLISHED", Pid: 99, Laddr: net.Addr{IP: "10.0.0.1", Port: 50006}, Raddr: net.Addr{IP: "203.0.113.10", Port: 443}},
		{Status: "ESTABLISHED", Pid: 1, Laddr: net.Addr{IP: "10.0.0.1", Port: 50007}, Raddr: net.Addr{IP: "127.0.0.1", Port: 443}},
		{Status: "ESTABLISHED", Pid: 1, Laddr: net.Addr{IP: "10.0.0.1", Port: 50008}, Raddr: net.Addr{IP: "10.0.0.2", Port: 443}},
		{Status: "ESTABLISHED", Pid: 3, Laddr: net.Addr{IP: "10.0.0.1", Port: 50009}, Raddr: net.Addr{IP: "203.0.113.10", Port: 443}},
		{Status: "ESTABLISHED", Pid: 1, Laddr: net.Addr{IP: "10.0.0.1", Port: 50010}, Raddr: net.Addr{IP: "203.0.113.10", Port: 8443}},
		{Status: "LISTEN", Laddr: net.Addr{IP: "10.0.0.1", Port: 60000}},
		{Status: "ESTABLISHED", Pid: 1, Laddr: net.Addr{IP: "10.0.0.1", Port: 60000}, Raddr: net.Addr{IP: "203.0.113.10", Port: 443}},
	})

	require.Equal(t, 1, md.MetricCount())
	metric := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "system.network.connection.count", metric.Name())
	dps := metric.Gauge().DataPoints()
	require.Equal(t, 1, dps.Len())
	dp := dps.At(0)
	assert.Equal(t, int64(2), dp.IntValue())
	assertAttributeValue(t, dp.Attributes(), "process.name", "app")
	assertAttributeValue(t, dp.Attributes(), "server.address", "203.0.113.10")
	serverPort, ok := dp.Attributes().Get("server.port")
	require.True(t, ok)
	assert.Equal(t, int64(443), serverPort.Int())
	assert.Equal(t, 1, processNameCalls[1])
}

func TestScrapeNetworkConnectionCountDisabledSkipsDetailLookups(t *testing.T) {
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.SystemNetworkConnectionCount.Enabled = false

	scraper, err := newNetworkScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &Config{MetricsBuilderConfig: cfg})
	require.NoError(t, err)
	scraper.bootTime = func(context.Context) (uint64, error) { return 100, nil }
	scraper.ioCounters = func(context.Context, bool) ([]net.IOCountersStat, error) { return nil, nil }
	scraper.conntrack = func(context.Context) ([]net.FilterStat, error) { return nil, nil }
	scraper.connections = func(context.Context, string) ([]net.ConnectionStat, error) {
		return []net.ConnectionStat{{Status: "ESTABLISHED", Pid: 1, Raddr: net.Addr{IP: "203.0.113.10", Port: 443}}}, nil
	}
	scraper.processName = func(context.Context, int32) (string, error) { panic("processName should not be called") }
	scraper.interfaceAddrs = func() ([]stdnet.Addr, error) { panic("interfaceAddrs should not be called") }

	require.NoError(t, scraper.start(t.Context(), componenttest.NewNopHost()))
	md, err := scraper.scrape(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, md.MetricCount())
	assert.Equal(t, "system.network.connections", md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Name())
}

func TestScrapeNetworkConnectionsErrorCountsEnabledMetrics(t *testing.T) {
	cfg := Config{MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig()}
	cfg.Metrics.SystemNetworkConnectionCount.Enabled = true

	scraper, err := newNetworkScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &cfg)
	require.NoError(t, err)
	scraper.bootTime = func(context.Context) (uint64, error) { return 100, nil }
	scraper.ioCounters = func(context.Context, bool) ([]net.IOCountersStat, error) { return nil, nil }
	scraper.conntrack = func(context.Context) ([]net.FilterStat, error) { return nil, nil }
	scraper.connections = func(context.Context, string) ([]net.ConnectionStat, error) { return nil, errors.New("err3") }

	require.NoError(t, scraper.start(t.Context(), componenttest.NewNopHost()))
	_, err = scraper.scrape(t.Context())
	require.EqualError(t, err, "failed to read TCP connections: err3")
	var scraperErr scrapererror.PartialScrapeError
	require.ErrorAs(t, err, &scraperErr)
	assert.Equal(t, 2, scraperErr.Failed)
}

func scrapeConnectionCountMetric(t *testing.T, cfg *Config, connections []net.ConnectionStat) (pmetric.Metrics, map[int32]int) {
	scraper, err := newNetworkScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)

	processNameCalls := map[int32]int{}
	scraper.bootTime = func(context.Context) (uint64, error) { return 100, nil }
	scraper.ioCounters = func(context.Context, bool) ([]net.IOCountersStat, error) { return nil, nil }
	scraper.conntrack = func(context.Context) ([]net.FilterStat, error) { return nil, nil }
	scraper.connections = func(context.Context, string) ([]net.ConnectionStat, error) { return connections, nil }
	scraper.processName = func(_ context.Context, pid int32) (string, error) {
		processNameCalls[pid]++
		switch pid {
		case 1:
			return "app", nil
		case 3:
			return "skip", nil
		default:
			return "", errors.New("process not found")
		}
	}
	scraper.interfaceAddrs = func() ([]stdnet.Addr, error) {
		return []stdnet.Addr{&stdnet.IPNet{IP: stdnet.ParseIP("10.0.0.2")}}, nil
	}

	require.NoError(t, scraper.start(t.Context(), componenttest.NewNopHost()))
	md, err := scraper.scrape(t.Context())
	require.NoError(t, err)
	return md, processNameCalls
}

func assertAttributeValue(t *testing.T, attrs pcommon.Map, name string, expected string) {
	value, ok := attrs.Get(name)
	require.True(t, ok)
	assert.Equal(t, expected, value.Str())
}
