// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zookeeperreceiver

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver/internal/metadata"
)

type logMsg struct {
	msg   string
	level zapcore.Level
}

func TestZookeeperMetricsScraperScrape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping flaky test on windows, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10171")
	}

	tests := []struct {
		name                        string
		expectedMetricsFilename     string
		expectedResourceAttributes  map[string]string
		metricsConfig               func() metadata.MetricsConfig
		mockedZKCmdToOutputFilename map[string]string
		mockZKConnectionErr         bool
		expectedLogs                []logMsg
		expectedNumResourceMetrics  int
		setConnectionDeadline       func(net.Conn, time.Time) error
		closeConnection             func(net.Conn) error
		sendCmd                     func(net.Conn, string) (*bufio.Scanner, error)
		wantErr                     bool
	}{
		{
			name: "Test correctness with v3.4.14",
			mockedZKCmdToOutputFilename: map[string]string{
				"mntr": "mntr-3.4.14",
				"ruok": "ruok-valid",
			},
			expectedMetricsFilename: "correctness-v3.4.14",
			expectedResourceAttributes: map[string]string{
				"server.state": "standalone",
				"zk.version":   "3.4.14-4c25d480e66aadd371de8bd2fd8da255ac140bcf",
			},
			expectedLogs: []logMsg{
				{
					msg:   "metric computation failed",
					level: zapcore.DebugLevel,
				},
			},
			expectedNumResourceMetrics: 1,
		},
		{
			name: "Test correctness with v3.5.5",
			mockedZKCmdToOutputFilename: map[string]string{
				"mntr": "mntr-3.5.5",
				"ruok": "ruok-valid",
			},
			expectedMetricsFilename: "correctness-v3.5.5",
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
			name: "Unexpected line format in mntr",
			mockedZKCmdToOutputFilename: map[string]string{
				"mntr": "mntr-unexpected_line_format",
				"ruok": "ruok-valid",
			},
			expectedLogs: []logMsg{
				{
					msg:   "unexpected line in response",
					level: zapcore.WarnLevel,
				},
				{
					msg:   "metric computation failed",
					level: zapcore.DebugLevel,
				},
			},
			expectedNumResourceMetrics: 0,
		},
		{
			name: "Unexpected value type in mntr",
			mockedZKCmdToOutputFilename: map[string]string{
				"mntr": "mntr-unexpected_value_type",
				"ruok": "ruok-valid",
			},
			expectedLogs: []logMsg{
				{
					msg:   "non-integer value from mntr",
					level: zapcore.DebugLevel,
				},
				{
					msg:   "metric computation failed",
					level: zapcore.DebugLevel,
				},
			},
			expectedNumResourceMetrics: 0,
		},
		{
			name: "Empty response from ruok",
			mockedZKCmdToOutputFilename: map[string]string{
				"mntr": "mntr-3.4.14",
				"ruok": "ruok-null",
			},
			expectedMetricsFilename: "null-ruok",
			expectedLogs: []logMsg{
				{
					msg:   "metric computation failed",
					level: zapcore.DebugLevel,
				},
			},
			expectedNumResourceMetrics: 2,
		},
		{
			name: "Invalid response from ruok",
			mockedZKCmdToOutputFilename: map[string]string{
				"mntr": "mntr-3.4.14",
				"ruok": "ruok-invalid",
			},
			expectedMetricsFilename: "invalid-ruok",
			expectedLogs: []logMsg{
				{
					msg:   "metric computation failed",
					level: zapcore.DebugLevel,
				},
				{
					msg:   "invalid response from ruok",
					level: zapcore.ErrorLevel,
				},
			},
			expectedNumResourceMetrics: 2,
		},
		{
			name: "Error setting connection deadline",
			mockedZKCmdToOutputFilename: map[string]string{
				"mntr": "mntr-3.4.14",
				"ruok": "ruok-valid",
			},
			expectedLogs: []logMsg{
				{
					msg:   "failed to set deadline on connection",
					level: zapcore.WarnLevel,
				},
				{
					msg:   "failed to set deadline on connection",
					level: zapcore.WarnLevel,
				},
				{
					msg:   "metric computation failed",
					level: zapcore.DebugLevel,
				},
			},
			expectedMetricsFilename: "error-setting-connection-deadline",
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
			name: "Error closing connection",
			mockedZKCmdToOutputFilename: map[string]string{
				"mntr": "mntr-3.4.14",
				"ruok": "ruok-valid",
			},
			expectedLogs: []logMsg{
				{
					msg:   "failed to shutdown connection",
					level: zapcore.WarnLevel,
				},
				{
					msg:   "failed to shutdown connection",
					level: zapcore.WarnLevel,
				},
				{
					msg:   "metric computation failed",
					level: zapcore.DebugLevel,
				},
			},
			expectedMetricsFilename: "error-closing-connection",
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
			name: "Failed to send command",
			mockedZKCmdToOutputFilename: map[string]string{
				"mntr": "mntr-3.4.14",
				"ruok": "ruok-valid",
			},
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
		{
			name: "Disable zookeeper.watches metric",
			metricsConfig: func() metadata.MetricsConfig {
				ms := metadata.DefaultMetricsConfig()
				ms.ZookeeperWatchCount.Enabled = false
				return ms
			},
			mockedZKCmdToOutputFilename: map[string]string{
				"mntr": "mntr-3.4.14",
				"ruok": "ruok-valid",
			},
			expectedMetricsFilename: "disable-watches",
			expectedResourceAttributes: map[string]string{
				"server.state": "standalone",
				"zk.version":   "3.4.14-4c25d480e66aadd371de8bd2fd8da255ac140bcf",
			},
			expectedLogs: []logMsg{
				{
					msg:   "metric computation failed",
					level: zapcore.DebugLevel,
				},
			},
			expectedNumResourceMetrics: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localAddr := testutil.GetAvailableLocalAddress(t)
			if !tt.mockZKConnectionErr {
				ms := mockedServer{ready: make(chan bool, 1)}
				go ms.mockZKServer(t, localAddr, tt.mockedZKCmdToOutputFilename)
				<-ms.ready
			}

			cfg := createDefaultConfig().(*Config)
			cfg.TCPAddr.Endpoint = localAddr
			if tt.metricsConfig != nil {
				cfg.MetricsBuilderConfig.Metrics = tt.metricsConfig()
			}

			core, observedLogs := observer.New(zap.DebugLevel)
			settings := receivertest.NewNopCreateSettings()
			settings.Logger = zap.New(core)
			z, err := newZookeeperMetricsScraper(settings, cfg)
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

			actualMetrics, err := z.scrape(ctx)
			require.NoError(t, z.shutdown(ctx))

			require.Equal(t, len(tt.expectedLogs), observedLogs.Len())
			for i, log := range tt.expectedLogs {
				require.Equal(t, log.msg, observedLogs.All()[i].Message)
				require.Equal(t, log.level, observedLogs.All()[i].Level)
			}

			if tt.expectedNumResourceMetrics == 0 {
				if tt.wantErr {
					require.Error(t, err)
					require.Equal(t, pmetric.NewMetrics(), actualMetrics)
				}
				require.NoError(t, z.shutdown(ctx))
				return
			}

			expectedFile := filepath.Join("testdata", "scraper", fmt.Sprintf("%s.yaml", tt.expectedMetricsFilename))
			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
				pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
		})
	}
}

func TestZookeeperShutdownBeforeScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	z, err := newZookeeperMetricsScraper(receivertest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)
	require.NoError(t, z.shutdown(context.Background()))
}

type mockedServer struct {
	ready chan bool
}

func (ms *mockedServer) mockZKServer(t *testing.T, endpoint string, cmdToFileMap map[string]string) {
	var cmd string
	listener, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer listener.Close()
	ms.ready <- true

	for {
		conn, err := listener.Accept()
		require.NoError(t, err)
		reader := bufio.NewReader(conn)
		scanner := bufio.NewScanner(reader)
		scanner.Scan()
		if cmd = scanner.Text(); cmd == "" {
			continue
		}

		require.NoError(t, err)
		filename := cmdToFileMap[cmd]
		out, err := os.ReadFile(filepath.Join("testdata", filename))
		require.NoError(t, err)

		_, err = conn.Write(out)
		require.NoError(t, err)

		conn.Close()

	}
}
