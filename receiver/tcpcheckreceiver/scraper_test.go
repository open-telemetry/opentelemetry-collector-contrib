// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

type Server struct {
	host     string
	port     string
	listener net.Listener
}

func newTCPServer(host string, port string) *Server {
	return &Server{
		host: host,
		port: port,
	}
}

func (server *Server) runTCPServer(t *testing.T) string {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", server.host, server.port))
	require.NoError(t, err)
	server.listener = listener
	go func() {
		conn, err := listener.Accept()
		assert.NoError(t, err)
		go handleRequest(conn)
	}()
	return listener.Addr().String()
}

func (server *Server) runTCPServerError() (string, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", server.host, server.port))
	if err != nil {
		return "", err
	}
	server.listener = listener
	go func() {
		time.Sleep(time.Millisecond * 100)
		err := listener.Close()
		if err != nil {
			fmt.Printf("Error closing listener: %v\n", err)
		}
	}()
	return listener.Addr().String(), nil
}

func (server *Server) shutdown() {
	server.listener.Close()
}

func handleRequest(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return
		}
		fmt.Printf("Message incoming: %s", message)
		_, err = conn.Write([]byte("Message received.\n"))
		if err != nil {
			conn.Close()
			return
		}
		conn.Close()
	}
}

func timeout(deadline time.Time, timeout time.Duration) time.Duration {
	timeToDeadline := time.Until(deadline)
	if timeToDeadline < timeout {
		return timeToDeadline
	}
	return timeout
}

func TestTimeout(t *testing.T) {
	testCases := []struct {
		name     string
		deadline time.Time
		timeout  time.Duration
		want     time.Duration
	}{
		{
			name:     "timeout is shorter",
			deadline: time.Now().Add(time.Second),
			timeout:  time.Second * 2,
			want:     time.Second,
		},
		{
			name:     "deadline is shorter",
			deadline: time.Now().Add(time.Second * 2),
			timeout:  time.Second,
			want:     time.Second,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			to := timeout(tc.deadline, tc.timeout)
			if to < (tc.want-10*time.Millisecond) || to > tc.want {
				t.Fatalf("wanted time within 10 milliseconds: %s, got: %s", time.Second, to)
			}
		})
	}
}

func TestScraper(t *testing.T) {
	s := newTCPServer("127.0.0.1", "8080")
	endpoint := s.runTCPServer(t)
	defer s.shutdown()
	testCases := []struct {
		name     string
		filename string
		endpoint string
	}{
		{
			name:     "metrics_golden",
			filename: "expected.yaml",
			endpoint: endpoint,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedFile := filepath.Join("testdata", "expected_metrics", tc.filename)
			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)
			cfg := &Config{
				Targets: []*confignet.TCPAddrConfig{
					{
						Endpoint: "127.0.0.1:8080",
						DialerConfig: confignet.DialerConfig{
							Timeout: 3 * time.Second,
						},
					},
				},
			}

			cfg.CollectionInterval = 100 * time.Millisecond
			settings := receivertest.NewNopSettings(metadata.Type)

			scraper := newScraper(cfg, settings)
			actualMetrics, err := scraper.scrape(context.Background())
			actualMetrics.ResourceMetrics()
			require.NoError(t, err, "failed scrape")
			require.NoError(
				t,
				pmetrictest.CompareMetrics(
					expectedMetrics,
					actualMetrics,
					pmetrictest.IgnoreMetricValues("tcpcheck.duration"),
					pmetrictest.IgnoreTimestamp(),
					pmetrictest.IgnoreStartTimestamp(),
				),
			)
		})
	}
}

func TestScraper_TCPErrorMetrics(t *testing.T) {
	s := newTCPServer("127.0.0.1", "8081")
	endpoint, _ := s.runTCPServerError()
	defer s.shutdown()

	testCases := []struct {
		name     string
		filename string
		endpoint string
	}{
		{
			name:     "tcp_error_metrics",
			filename: "expected_error.yaml",
			endpoint: endpoint,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedFile := filepath.Join("testdata", "expected_metrics", tc.filename)
			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)

			if runtime.GOOS == "windows" {
				expectedError := "dial tcp 127.0.0.1:9999: connectex: No connection could be made because the target machine actively refused it."
				expectedMetrics = updateErrorCodeInMetrics(expectedMetrics, expectedError)
			}

			cfg := &Config{
				Targets: []*confignet.TCPAddrConfig{
					{
						Endpoint: "127.0.0.1:9999",
						DialerConfig: confignet.DialerConfig{
							Timeout: 3 * time.Second,
						},
					},
				},
			}

			cfg.CollectionInterval = 100 * time.Millisecond
			settings := receivertest.NewNopSettings(metadata.Type)

			scraper := newScraper(cfg, settings)
			actualMetrics, err := scraper.scrape(context.Background())
			require.NoError(t, err, "failed scrape")

			require.NoError(
				t,
				pmetrictest.CompareMetrics(
					expectedMetrics,
					actualMetrics,
					pmetrictest.IgnoreTimestamp(),
					pmetrictest.IgnoreStartTimestamp(),
				),
			)
		})
	}
}

func updateErrorCodeInMetrics(metrics pmetric.Metrics, newErrorCode string) pmetric.Metrics {
	metrics.ResourceMetrics().At(0).
		ScopeMetrics().At(0).
		Metrics().At(0).
		Sum().DataPoints().At(0).
		Attributes().PutStr("error.code", newErrorCode)
	return metrics
}
