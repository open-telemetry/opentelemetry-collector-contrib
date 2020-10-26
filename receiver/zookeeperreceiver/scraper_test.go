// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zookeeperreceiver

import (
	"context"
	"io/ioutil"
	"net"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type logMsg struct {
	msg   string
	level zapcore.Level
}

func TestZookeeperMetricsScraperScrape(t *testing.T) {
	commonMetrics := []pdata.Metric{
		avgLatencyDescriptor,
		maxLatencyDescriptor,
		minLatencyDescriptor,
		numAliveConnectionsDescriptor,
		outstandingRequestsDescriptor,
		zNodeCountDescriptor,
		watchCountDescriptor,
		ephemeralsCountDescriptor,
		approximateDataSizeDescriptor,
		openFileDescriptorCountDescriptor,
		maxFileDescriptorCountDescriptor,
	}
	localAddr := testutil.GetAvailableLocalAddress(t)
	tests := []struct {
		name                         string
		expectedMetrics              []pdata.Metric
		expectedResourceAttributes   map[string]string
		mockedZKOutputSourceFilename string
		mockZKConnectionErr          bool
		expectedLogs                 []logMsg
		wantErr                      bool
	}{
		{
			name:                         "Test correctness with v3.4.14",
			mockedZKOutputSourceFilename: "mntr-3.4.14",
			expectedMetrics: func() []pdata.Metric {
				out := make([]pdata.Metric, 0, len(commonMetrics)+1)
				for _, metric := range commonMetrics {
					out = append(out, metric)
				}

				return out
			}(),
			expectedResourceAttributes: map[string]string{
				"server.state": "standalone",
			},
		},
		{
			name:                         "Test correctness with v3.5.5",
			mockedZKOutputSourceFilename: "mntr-3.5.5",
			expectedMetrics: func() []pdata.Metric {
				out := make([]pdata.Metric, 0, len(commonMetrics)+3)
				for _, metric := range commonMetrics {
					out = append(out, metric)
				}

				for _, metric := range []pdata.Metric{
					followersDescriptor,
					syncedFollowersDescriptor,
					pendingSyncsDescriptor,
				} {
					out = append(out, metric)
				}
				return out
			}(),
			expectedResourceAttributes: map[string]string{
				"server.state": "leader",
			},
		},
		{
			name:                "Arbitrary connection error",
			mockZKConnectionErr: true,
			expectedLogs: []logMsg{
				{
					msg:   "failed to establish connection",
					level: zapcore.ErrorLevel,
				},
			},
			wantErr: true,
		},
		{
			name:                         "Unexpected line format in mntr",
			mockedZKOutputSourceFilename: "mntr-unexpected_line_format",
			expectedLogs: []logMsg{
				{
					msg:   "unexpected line in response",
					level: zapcore.WarnLevel,
				},
			},
		},
		{
			name:                         "Unexpected value type in mntr",
			mockedZKOutputSourceFilename: "mntr-unexpected_value_type",
			expectedLogs: []logMsg{
				{
					msg:   "non-integer value from mntr",
					level: zapcore.DebugLevel,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.mockZKConnectionErr {
				go mockZKServer(t, localAddr, tt.mockedZKOutputSourceFilename)
			}
			time.Sleep(100 * time.Millisecond)

			cfg := &Config{
				TCPAddr: confignet.TCPAddr{
					Endpoint: localAddr,
				},
				Timeout: defaultTimeout,
			}

			core, observedLogs := observer.New(zap.DebugLevel)
			z := &zookeeperMetricsScraper{
				logger: zap.New(core),
				config: cfg,
			}
			ctx := context.Background()
			require.NoError(t, z.Initialize(ctx))

			got, err := z.Scrape(ctx)
			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, pdata.ResourceMetricsSlice{}, got)

				return
			}

			require.Equal(t, 1, got.Len())

			resource := got.At(0).Resource()
			resource.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
				require.Equal(t, tt.expectedResourceAttributes[k], v.StringVal())
			})

			ilms := got.At(0).InstrumentationLibraryMetrics()
			require.Equal(t, 1, ilms.Len())

			metrics := ilms.At(0).Metrics()
			require.Equal(t, len(tt.expectedMetrics), metrics.Len())

			for i, metric := range tt.expectedMetrics {
				assertMetricValid(t, metrics.At(i), metric)
			}

			require.Equal(t, len(tt.expectedLogs), observedLogs.Len())
			for i, log := range tt.expectedLogs {
				require.Equal(t, log.msg, observedLogs.All()[i].Message)
				require.Equal(t, log.level, observedLogs.All()[i].Level)
			}

			require.NoError(t, z.Close(ctx))
		})
	}
}

func assertMetricValid(t *testing.T, metric pdata.Metric, descriptor pdata.Metric) {
	assertDescriptorEqual(t, descriptor, metric)
	require.GreaterOrEqual(t, metric.IntGauge().DataPoints().Len(), 1)
}

func assertDescriptorEqual(t *testing.T, expected pdata.Metric, actual pdata.Metric) {
	require.Equal(t, expected.Name(), actual.Name())
	require.Equal(t, expected.Description(), actual.Description())
	require.Equal(t, expected.Unit(), actual.Unit())
	require.Equal(t, expected.DataType(), actual.DataType())
}

func mockZKServer(t *testing.T, endpoint string, filename string) {
	listener, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer listener.Close()

	conn, err := listener.Accept()
	require.NoError(t, err)

	for {
		out, err := ioutil.ReadFile(path.Join(".", "testdata", filename))
		require.NoError(t, err)

		conn.Write(out)
		conn.Close()
		return
	}
}
