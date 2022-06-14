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

package networkscraper

import (
	"context"
	"errors"
	"testing"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	type testCase struct {
		name                                   string
		config                                 Config
		bootTimeFunc                           func() (uint64, error)
		ioCountersFunc                         func(bool) ([]net.IOCountersStat, error)
		connectionsFunc                        func(string) ([]net.ConnectionStat, error)
		expectNetworkMetrics                   bool
		expectedStartTime                      pcommon.Timestamp
		newErrRegex                            string
		initializationErr                      string
		expectedErr                            string
		expectedErrCount                       int
		expectMetricsWithDirectionAttribute    bool
		expectMetricsWithoutDirectionAttribute bool
		expectMetricsWithProtocolAttribute     bool
		expectMetricsWithoutProtocolAttribute  bool
		mutateScraper                          func(*scraper)
	}

	testCases := []testCase{
		{
			name: "Standard",
			config: Config{
				Metrics: metadata.DefaultMetricsSettings(),
			},
			expectNetworkMetrics:                true,
			expectMetricsWithDirectionAttribute: true,
			expectMetricsWithProtocolAttribute:  true,
		},
		{
			name: "Standard with direction removed",
			config: Config{
				Metrics: metadata.DefaultMetricsSettings(),
			},
			expectNetworkMetrics:                   true,
			expectMetricsWithDirectionAttribute:    false,
			expectMetricsWithoutDirectionAttribute: true,
			expectMetricsWithProtocolAttribute:     true,
			mutateScraper: func(s *scraper) {
				s.emitMetricsWithDirectionAttribute = false
				s.emitMetricsWithoutDirectionAttribute = true
			},
		},
		{
			name: "Standard with protocol removed",
			config: Config{
				Metrics: metadata.DefaultMetricsSettings(),
			},
			expectNetworkMetrics:                  true,
			expectMetricsWithProtocolAttribute:    false,
			expectMetricsWithoutProtocolAttribute: true,
			expectMetricsWithDirectionAttribute:   true,
			mutateScraper: func(s *scraper) {
				s.emitMetricsWithProtocolAttribute = false
				s.emitMetricsWithoutProtocolAttribute = true
			},
		},
		{
			name: "Validate Start Time",
			config: Config{
				Metrics: metadata.DefaultMetricsSettings(),
			},
			bootTimeFunc:                        func() (uint64, error) { return 100, nil },
			expectNetworkMetrics:                true,
			expectMetricsWithDirectionAttribute: true,
			expectMetricsWithProtocolAttribute:  true,
			expectedStartTime:                   100 * 1e9,
		},
		{
			name: "Include Filter that matches nothing",
			config: Config{
				Metrics: metadata.DefaultMetricsSettings(),
				Include: MatchConfig{filterset.Config{MatchType: "strict"}, []string{"@*^#&*$^#)"}},
			},
			expectNetworkMetrics:               false,
			expectMetricsWithProtocolAttribute: true,
		},
		{
			name: "Invalid Include Filter",
			config: Config{
				Metrics: metadata.DefaultMetricsSettings(),
				Include: MatchConfig{Interfaces: []string{"test"}},
			},
			newErrRegex: "^error creating network interface include filters:",
		},
		{
			name: "Invalid Exclude Filter",
			config: Config{
				Metrics: metadata.DefaultMetricsSettings(),
				Exclude: MatchConfig{Interfaces: []string{"test"}},
			},
			newErrRegex: "^error creating network interface exclude filters:",
		},
		{
			name:              "Boot Time Error",
			bootTimeFunc:      func() (uint64, error) { return 0, errors.New("err1") },
			initializationErr: "err1",
		},
		{
			name:             "IOCounters Error",
			ioCountersFunc:   func(bool) ([]net.IOCountersStat, error) { return nil, errors.New("err2") },
			expectedErr:      "failed to read network IO stats: err2",
			expectedErrCount: networkMetricsLen,
		},
		{
			name:             "Connections Error",
			connectionsFunc:  func(string) ([]net.ConnectionStat, error) { return nil, errors.New("err3") },
			expectedErr:      "failed to read TCP connections: err3",
			expectedErrCount: connectionsMetricsLen,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper, err := newNetworkScraper(context.Background(), componenttest.NewNopReceiverCreateSettings(), &test.config)
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

			expectedMetricCount := 1
			if test.expectNetworkMetrics {
				if test.expectMetricsWithoutDirectionAttribute {
					expectedMetricCount += 8
				}
				if test.expectMetricsWithDirectionAttribute {
					expectedMetricCount += 4
				}
				if test.expectMetricsWithoutProtocolAttribute {
					expectedMetricCount += 1
				}
			}
			assert.Equal(t, expectedMetricCount, md.MetricCount())

			metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			// these idx are used to compare timestamps between network metrics.
			// the value of the index was previously hardcoded
			startIdx, endIdx := 0, 0
			for idx := 0; idx < metrics.Len(); idx++ {
				metric := metrics.At(idx)
				switch metric.Name() {
				case "system.network.connections":
					assert.True(t, test.expectMetricsWithProtocolAttribute)
					assertTCPConnectionsMetricValid(t, metric, metric.Name(), false, true)
				case "system.network.tcp.connections":
					assert.True(t, test.expectMetricsWithoutProtocolAttribute)
					assertTCPConnectionsMetricValid(t, metric, metric.Name(), true, true)
				case "system.network.udp.connections":
					assert.True(t, test.expectMetricsWithoutProtocolAttribute)
					assertNetworkConnectionsMetricValid(t, metric, metric.Name(), true, false)
				case "system.network.dropped", "system.network.errors", "system.network.io", "system.network.packets":
					if startIdx == 0 {
						startIdx = idx
					}
					endIdx = idx
					assert.True(t, test.expectMetricsWithDirectionAttribute)
					assertNetworkIOMetricValid(t, metric, metric.Name(), test.expectedStartTime, false)
				case "system.network.dropped.receive", "system.network.dropped.transmit", "system.network.errors.receive",
					"system.network.errors.transmit", "system.network.io.receive", "system.network.io.transmit",
					"system.network.packets.receive", "system.network.packets.transmit":
					if startIdx == 0 {
						startIdx = idx
					}
					endIdx = idx
					assert.True(t, test.expectMetricsWithoutDirectionAttribute)
					assertNetworkIOMetricValid(t, metric, metric.Name(), test.expectedStartTime, true)
				default:
					assert.Fail(t, "unexpected metric", map[string]string{"metric name": metric.Name()})
				}
			}
			if test.expectNetworkMetrics {
				internal.AssertSameTimeStampForMetrics(t, metrics, startIdx, endIdx)
			}
		})
	}
}

func assertNetworkIOMetricValid(t *testing.T, metric pmetric.Metric, expectedName string, startTime pcommon.Timestamp, expectDirectionRemoved bool) {
	assert.Equal(t, expectedName, metric.Name())
	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, metric, startTime)
	}
	if expectDirectionRemoved {
		assert.GreaterOrEqual(t, metric.Sum().DataPoints().Len(), 1)
	} else {
		assert.GreaterOrEqual(t, metric.Sum().DataPoints().Len(), 2)
	}
	internal.AssertSumMetricHasAttribute(t, metric, 0, "device")
	if !expectDirectionRemoved {
		internal.AssertSumMetricHasAttribute(t, metric, 0, "direction")
	}
}

func assertNetworkConnectionsMetricValid(t *testing.T, metric pmetric.Metric, expectedName string, expectProtocolRemoved bool, expectState bool) {
	assert.Equal(t, metric.Name(), expectedName)
	if expectState {
		internal.AssertSumMetricHasAttribute(t, metric, 0, "state")
	}

	if !expectProtocolRemoved {
		internal.AssertSumMetricHasAttributeValue(t, metric, 0, "protocol",
			pcommon.NewValueString(metadata.AttributeProtocolTcp.String()))
	}
}

func assertTCPConnectionsMetricValid(t *testing.T, metric pmetric.Metric, expectedName string, expectProtocolRemoved bool, expectState bool) {
	assert.Equal(t, 12, metric.Sum().DataPoints().Len())
	assertNetworkConnectionsMetricValid(t, metric, expectedName, expectProtocolRemoved, expectState)
}
