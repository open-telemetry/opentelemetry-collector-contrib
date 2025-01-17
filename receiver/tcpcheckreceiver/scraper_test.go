package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"bufio"
	"context"
	"fmt"
	"go.opentelemetry.io/collector/config/confignet"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
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

func (server *Server) runTCPServerError(t *testing.T) (string, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", server.host, server.port))
	if err != nil {
		return "", err // Return the error if Listen fails
	}
	server.listener = listener
	go func() {
		// Simulate a TCP error condition
		time.Sleep(time.Millisecond * 100) // Simulate some delay before closing
		err := listener.Close()
		if err != nil {
			// Log the error safely without using the test logger
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

			cfg.ControllerConfig.CollectionInterval = 100 * time.Millisecond
			settings := receivertest.NewNopSettings()

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

func TestScraper_TCPErrorMetrics(t *testing.T) {
	// Start a TCP server on port 8080
	s := newTCPServer("127.0.0.1", "8081")
	endpoint, _ := s.runTCPServerError(t)
	defer s.shutdown()

	testCases := []struct {
		name     string
		filename string
		endpoint string
	}{
		{
			name:     "tcp_error_metrics",
			filename: "expected_error.yaml", // Expected metrics file containing tcp.error
			endpoint: endpoint,              // Correct endpoint
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
						Endpoint: "127.0.0.1:9999", // Correct endpoint
						DialerConfig: confignet.DialerConfig{
							Timeout: 3 * time.Second,
						},
					},
				},
			}

			cfg.ControllerConfig.CollectionInterval = 100 * time.Millisecond
			settings := receivertest.NewNopSettings()

			scraper := newScraper(cfg, settings)
			actualMetrics, err := scraper.scrape(context.Background())
			require.NoError(t, err, "failed scrape")

			// Compare the actual metrics to the expected metrics, including tcp.error
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
