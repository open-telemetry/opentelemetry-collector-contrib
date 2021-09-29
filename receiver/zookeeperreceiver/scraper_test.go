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
	"bufio"
	"context"
	"errors"
	"io/ioutil"
	"net"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver/internal/metadata"
)

type logMsg struct {
	msg   string
	level zapcore.Level
}

func TestZookeeperMetricsScraperScrape(t *testing.T) {
	commonMetrics := []pdata.Metric{
		metadata.Metrics.ZookeeperLatencyAvg.New(),
		metadata.Metrics.ZookeeperLatencyMax.New(),
		metadata.Metrics.ZookeeperLatencyMin.New(),
		metadata.Metrics.ZookeeperPacketsReceived.New(),
		metadata.Metrics.ZookeeperPacketsSent.New(),
		metadata.Metrics.ZookeeperConnectionsAlive.New(),
		metadata.Metrics.ZookeeperOutstandingRequests.New(),
		metadata.Metrics.ZookeeperZnodes.New(),
		metadata.Metrics.ZookeeperWatches.New(),
		metadata.Metrics.ZookeeperEphemeralNodes.New(),
		metadata.Metrics.ZookeeperApproximateDateSize.New(),
		metadata.Metrics.ZookeeperOpenFileDescriptors.New(),
		metadata.Metrics.ZookeeperMaxFileDescriptors.New(),
	}

	var metricsV3414 []pdata.Metric
	metricsV3414 = append(metricsV3414, commonMetrics...)
	metricsV3414 = append(metricsV3414, metadata.Metrics.ZookeeperFsyncThresholdExceeds.New())

	tests := []struct {
		name                         string
		expectedMetrics              []pdata.Metric
		expectedResourceAttributes   map[string]string
		mockedZKOutputSourceFilename string
		mockZKConnectionErr          bool
		expectedLogs                 []logMsg
		expectedNumResourceMetrics   int
		setConnectionDeadline        func(net.Conn, time.Time) error
		closeConnection              func(net.Conn) error
		sendCmd                      func(net.Conn, string) (*bufio.Scanner, error)
		wantErr                      bool
	}{
		{
			name:                         "Test correctness with v3.4.14",
			mockedZKOutputSourceFilename: "mntr-3.4.14",
			expectedMetrics:              metricsV3414,
			expectedResourceAttributes: map[string]string{
				"server.state": "standalone",
				"zk.version":   "3.4.14-4c25d480e66aadd371de8bd2fd8da255ac140bcf",
			},
			expectedNumResourceMetrics: 1,
		},
		{
			name:                         "Test correctness with v3.5.5",
			mockedZKOutputSourceFilename: "mntr-3.5.5",
			expectedMetrics: func() []pdata.Metric {
				out := make([]pdata.Metric, 0, len(commonMetrics)+3)
				out = append(out, commonMetrics...)

				out = append(out, []pdata.Metric{
					metadata.Metrics.ZookeeperFollowers.New(),
					metadata.Metrics.ZookeeperSyncedFollowers.New(),
					metadata.Metrics.ZookeeperPendingSyncs.New(),
				}...)
				return out
			}(),
			expectedResourceAttributes: map[string]string{
				"server.state": "leader",
				"zk.version":   "3.5.5-390fe37ea45dee01bf87dc1c042b5e3dcce88653",
			},
			expectedNumResourceMetrics: 1,
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
			expectedNumResourceMetrics: 0,
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
			expectedNumResourceMetrics: 0,
		},
		{
			name:                         "Error setting connection deadline",
			mockedZKOutputSourceFilename: "mntr-3.4.14",
			expectedLogs: []logMsg{
				{
					msg:   "failed to set deadline on connection",
					level: zapcore.WarnLevel,
				},
			},
			expectedMetrics: metricsV3414,
			expectedResourceAttributes: map[string]string{
				"server.state": "standalone",
				"zk.version":   "3.4.14-4c25d480e66aadd371de8bd2fd8da255ac140bcf",
			},
			expectedNumResourceMetrics: 1,
			setConnectionDeadline: func(conn net.Conn, t time.Time) error {
				return errors.New("")
			},
		},
		{
			name:                         "Error closing connection",
			mockedZKOutputSourceFilename: "mntr-3.4.14",
			expectedLogs: []logMsg{
				{
					msg:   "failed to shutdown connection",
					level: zapcore.WarnLevel,
				},
			},
			expectedMetrics: metricsV3414,
			expectedResourceAttributes: map[string]string{
				"server.state": "standalone",
				"zk.version":   "3.4.14-4c25d480e66aadd371de8bd2fd8da255ac140bcf",
			},
			expectedNumResourceMetrics: 1,
			closeConnection: func(conn net.Conn) error {
				return errors.New("")
			},
		},
		{
			name:                         "Failed to send command",
			mockedZKOutputSourceFilename: "mntr-3.4.14",
			expectedLogs: []logMsg{
				{
					msg:   "failed to send command",
					level: zapcore.ErrorLevel,
				},
			},
			sendCmd: func(conn net.Conn, s string) (*bufio.Scanner, error) {
				return nil, errors.New("")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localAddr := testutil.GetAvailableLocalAddress(t)
			if !tt.mockZKConnectionErr {
				ms := mockedServer{ready: make(chan bool, 1)}
				go ms.mockZKServer(t, localAddr, tt.mockedZKOutputSourceFilename)
				<-ms.ready
			}

			cfg := &Config{
				TCPAddr: confignet.TCPAddr{
					Endpoint: localAddr,
				},
				Timeout: defaultTimeout,
			}

			core, observedLogs := observer.New(zap.DebugLevel)
			z, err := newZookeeperMetricsScraper(zap.New(core), cfg)
			require.NoError(t, err)
			require.Equal(t, "zookeeper", z.Name())

			ctx := context.Background()

			if tt.setConnectionDeadline != nil {
				z.setConnectionDeadline = tt.setConnectionDeadline
			}

			if tt.closeConnection != nil {
				z.closeConnection = tt.closeConnection
			}

			if tt.sendCmd != nil {
				z.sendCmd = tt.sendCmd
			}

			got, err := z.scrape(ctx)

			require.Equal(t, len(tt.expectedLogs), observedLogs.Len())
			for i, log := range tt.expectedLogs {
				require.Equal(t, log.msg, observedLogs.All()[i].Message)
				require.Equal(t, log.level, observedLogs.All()[i].Level)
			}

			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, pdata.NewResourceMetricsSlice(), got)

				require.NoError(t, z.shutdown(ctx))
				return
			}

			require.Equal(t, tt.expectedNumResourceMetrics, got.Len())
			for i := 0; i < tt.expectedNumResourceMetrics; i++ {
				resource := got.At(i).Resource()
				require.Equal(t, len(tt.expectedResourceAttributes), resource.Attributes().Len())
				resource.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
					require.Equal(t, tt.expectedResourceAttributes[k], v.StringVal())
					return true
				})

				ilms := got.At(0).InstrumentationLibraryMetrics()
				require.Equal(t, 1, ilms.Len())

				metrics := ilms.At(0).Metrics()
				require.Equal(t, len(tt.expectedMetrics), metrics.Len())

				for i, metric := range tt.expectedMetrics {
					assertMetricValid(t, metrics.At(i), metric)
				}
			}

			require.NoError(t, z.shutdown(ctx))
		})
	}
}

func assertMetricValid(t *testing.T, metric pdata.Metric, descriptor pdata.Metric) {
	assertDescriptorEqual(t, descriptor, metric)
	switch metric.DataType() {
	case pdata.MetricDataTypeGauge:
		require.GreaterOrEqual(t, metric.Gauge().DataPoints().Len(), 1)
	case pdata.MetricDataTypeSum:
		require.GreaterOrEqual(t, metric.Sum().DataPoints().Len(), 1)
	}
}

func assertDescriptorEqual(t *testing.T, expected pdata.Metric, actual pdata.Metric) {
	require.Equal(t, expected.Name(), actual.Name())
	require.Equal(t, expected.Description(), actual.Description())
	require.Equal(t, expected.Unit(), actual.Unit())
	require.Equal(t, expected.DataType(), actual.DataType())
}

type mockedServer struct {
	ready chan bool
}

func (ms *mockedServer) mockZKServer(t *testing.T, endpoint string, filename string) {
	listener, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer listener.Close()

	ms.ready <- true

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
