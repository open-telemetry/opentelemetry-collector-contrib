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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

type mockTCPserver struct {
	host     string
	port     string
	listener net.Listener
}

func newTCPServer(host string, port string) *mockTCPserver {
	return &mockTCPserver{
		host: host,
		port: port,
	}
}

func (server *mockTCPserver) runTCPServer(t *testing.T) string {
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

func (server *mockTCPserver) runTCPServerError() (string, error) {
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

func (server *mockTCPserver) shutdown() {
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
				expectedMetrics = updateErrorCodeInMetrics(expectedMetrics, "connection_refused")
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

			settings := receivertest.NewNopSettings(metadata.Type)
			scraper := newScraper(cfg, settings)

			// Initialize metrics builder
			scraper.mb = metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings)

			actualMetrics, err := scraper.scrape(context.Background())
			require.Error(t, err, "expected connection refused error")

			for i := 0; i < actualMetrics.ResourceMetrics().Len(); i++ {
				rm := actualMetrics.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						m := sm.Metrics().At(k)
						fmt.Printf("Metric %d: %s\n", k+1, m.Name())
						if m.Name() == "tcpcheck.error" {
							dps := m.Sum().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								dp := dps.At(l)
								if val, ok := dp.Attributes().Get("error.code"); ok {
									fmt.Printf("Error code: %s\n", val.Str())
								}
							}
						}
					}
				}
			}

			// Create a new metrics object with only the error metric
			filteredMetrics := pmetric.NewMetrics()
			actualRm := filteredMetrics.ResourceMetrics().AppendEmpty()
			// Ensure the resource map is empty to match the expected metrics
			actualRm.Resource().Attributes().Clear()
			actualSm := actualRm.ScopeMetrics().AppendEmpty()
			actualSm.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver")
			actualSm.Scope().SetVersion("latest")

			// Copy only the error metric
			for i := 0; i < actualMetrics.ResourceMetrics().Len(); i++ {
				rm := actualMetrics.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						m := sm.Metrics().At(k)
						if m.Name() == "tcpcheck.error" {
							m.CopyTo(actualSm.Metrics().AppendEmpty())
						}
					}
				}
			}

			require.NoError(
				t,
				pmetrictest.CompareMetrics(
					expectedMetrics,
					filteredMetrics,
					pmetrictest.IgnoreTimestamp(),
					pmetrictest.IgnoreStartTimestamp(),
				),
			)
		})
	}
}

func updateErrorCodeInMetrics(metrics pmetric.Metrics, errorCode string) pmetric.Metrics {
	metrics.ResourceMetrics().At(0).
		ScopeMetrics().At(0).
		Metrics().At(0).
		Sum().DataPoints().At(0).
		Attributes().PutStr("error.code", errorCode)
	return metrics
}

func TestScraper_ErrorEnumCounts(t *testing.T) {
	// Test multiple endpoints with different error types
	endpoints := []string{
		"invalid:host", // Invalid host format for invalid_endpoint
		"1.2.3.4:80",   // Unreachable IP for connection_timeout
		"invalid:host", // Another invalid_endpoint
		"1.2.3.4:80",   // Another connection_timeout
		"1.2.3.4:80",   // Another connection_timeout
	}

	cfg := &Config{
		Targets: make([]*confignet.TCPAddrConfig, len(endpoints)),
	}

	for i, endpoint := range endpoints {
		cfg.Targets[i] = &confignet.TCPAddrConfig{
			Endpoint: endpoint,
			DialerConfig: confignet.DialerConfig{
				Timeout: 1 * time.Second,
			},
		}
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	scraper := newScraper(cfg, settings)

	// Initialize metrics builder
	scraper.mb = metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings)

	// Run a single scrape to collect all errors
	actualMetrics, err := scraper.scrape(context.Background())
	require.Error(t, err, "expected errors from scrape")

	// Print all metrics for debugging
	fmt.Printf("\n=== Debug Metrics ===\n")
	for i := 0; i < actualMetrics.ResourceMetrics().Len(); i++ {
		rm := actualMetrics.ResourceMetrics().At(i)
		fmt.Printf("ResourceMetrics[%d]:\n", i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			fmt.Printf("  ScopeMetrics[%d]:\n", j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				fmt.Printf("    Metric: %s\n", m.Name())
				if m.Name() == "tcpcheck.error" {
					dps := m.Sum().DataPoints()
					fmt.Printf("      DataPoints: %d\n", dps.Len())
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						fmt.Printf("        DataPoint[%d]:\n", l)
						fmt.Printf("          Value: %d\n", dp.IntValue())
						dp.Attributes().Range(func(k string, v pcommon.Value) bool {
							fmt.Printf("          Attribute: %s = %s\n", k, v.AsString())
							return true
						})
					}
				}
			}
		}
	}

	// Count errors by type
	errorCounts := make(map[string]int64)
	for i := 0; i < actualMetrics.ResourceMetrics().Len(); i++ {
		rm := actualMetrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				if m.Name() == "tcpcheck.error" {
					dps := m.Sum().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						if val, ok := dp.Attributes().Get("error.code"); ok {
							errorCounts[val.Str()] = dp.IntValue()
						}
					}
				}
			}
		}
	}

	// Print error counts for debugging
	fmt.Printf("\n=== Error Counts ===\n")
	for code, count := range errorCounts {
		fmt.Printf("Error code '%s': %d\n", code, count)
	}

	// Verify specific error counts
	require.Equal(t, int64(2), errorCounts["invalid_endpoint"], "Expected 2 invalid_endpoint errors")
	require.Equal(t, int64(3), errorCounts["connection_timeout"], "Expected 3 connection_timeout errors")
}

func TestScraper_MultipleEndpoints_ErrorSum(t *testing.T) {
	endpoints := []string{
		"localhost:0",   // Invalid port
		"invalid:host",  // Invalid host format
		"missing:port:", // Invalid port format
	}

	cfg := &Config{
		Targets: []*confignet.TCPAddrConfig{
			{
				Endpoint: endpoints[0],
				DialerConfig: confignet.DialerConfig{
					Timeout: 1 * time.Second,
				},
			},
			{
				Endpoint: endpoints[1],
				DialerConfig: confignet.DialerConfig{
					Timeout: 1 * time.Second,
				},
			},
			{
				Endpoint: endpoints[2],
				DialerConfig: confignet.DialerConfig{
					Timeout: 1 * time.Second,
				},
			},
		},
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	scraper := newScraper(cfg, settings)

	// Simulate 5 failed scrapes
	for idx := 0; idx < 5; idx++ {
		metrics, err := scraper.scrape(context.Background())
		require.Error(t, err)
		require.NotNil(t, metrics)

		// Check error count after each scrape
		rmSlice := metrics.ResourceMetrics()
		require.NotZero(t, rmSlice.Len(), "Expected non-zero metrics after scrape %d", idx+1)

		errorCounts := make(map[string]int64)
		totalErrors := int64(0)

		for i := 0; i < rmSlice.Len(); i++ {
			rm := rmSlice.At(i)
			smSlice := rm.ScopeMetrics()
			for j := 0; j < smSlice.Len(); j++ {
				sm := smSlice.At(j)
				metrics := sm.Metrics()
				for k := 0; k < metrics.Len(); k++ {
					m := metrics.At(k)
					if m.Name() == "tcpcheck.error" {
						dps := m.Sum().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							if val, ok := dp.Attributes().Get("tcpcheck.endpoint"); ok {
								endpoint := val.Str()
								errorCounts[endpoint] = dp.IntValue()
								totalErrors += dp.IntValue()
							}
						}
					}
				}
			}
		}

		// Verify total errors
		require.Equal(t, int64(idx+1)*int64(len(endpoints)), totalErrors,
			"Expected total errors to be %d after %d failed scrapes", (idx+1)*len(endpoints), idx+1)
	}
}
