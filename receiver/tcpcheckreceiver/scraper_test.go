package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"bufio"
	"context"
	"fmt"
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

func timeout(deadline time.Time, timeout time.Duration) time.Duration {
	timeToDeadline := time.Until(deadline)
	if timeToDeadline < timeout {
		return timeToDeadline
	}
	return timeout
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
			filename: "metrics_golden.yaml",
			endpoint: endpoint,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedFile := filepath.Join("testdata", "expected_metrics", tc.filename)
			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)
			f := NewFactory()
			cfg := f.CreateDefaultConfig().(*Config)
			cfg.ControllerConfig.CollectionInterval = 100 * time.Millisecond
			//cfg.Endpoint = tc.endpoint

			settings := receivertest.NewNopSettings()

			scraper := newScraper(cfg, settings)
			//require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()), "failed starting scraper")

			actualMetrics, err := scraper.scrape(context.Background())
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

/*

//nolint:revive
func mockGetConnectionStateExpired(endpoint string) (tls.ConnectionState, error) {
	cert := &x509.Certificate{
		NotBefore: time.Now().Add(-48 * time.Hour),
		NotAfter:  time.Now().Add(-24 * time.Hour),
		Subject:   pkix.Name{CommonName: "expired.com"},
		Issuer:    pkix.Name{CommonName: "ExpiredIssuer"},
	}
	return tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}, nil
}

//nolint:revive
func mockGetConnectionStateNotYetValid(endpoint string) (tls.ConnectionState, error) {
	cert := &x509.Certificate{
		NotBefore: time.Now().Add(48 * time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		Subject:   pkix.Name{CommonName: "notyetvalid.com"},
		Issuer:    pkix.Name{CommonName: "NotYetValidIssuer"},
	}
	return tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}, nil
}

func TestScrape_ValidCertificate(t *testing.T) {
	cfg := &Config{
		Targets: []*confignet.TCPAddrConfig{
			{Endpoint: "example.com:443"},
		},
	}
	settings := receivertest.NewNopSettings()
	s := newScraper(cfg, settings, mockGetConnectionStateValid)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, metrics.DataPointCount())

	rm := metrics.ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics().At(0)
	metric := ilms.Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)

	attributes := dp.Attributes()
	issuer, _ := attributes.Get("tlscheck.x509.issuer")
	commonName, _ := attributes.Get("tlscheck.x509.cn")

	assert.Equal(t, "CN=ValidIssuer", issuer.AsString())
	assert.Equal(t, "valid.com", commonName.AsString())
}

func TestScrape_ExpiredCertificate(t *testing.T) {
	cfg := &Config{
		Targets: []*confignet.TCPAddrConfig{
			{Endpoint: "expired.com:443"},
		},
	}
	settings := receivertest.NewNopSettings()
	s := newScraper(cfg, settings, mockGetConnectionStateExpired)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, metrics.DataPointCount())

	rm := metrics.ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics().At(0)
	metric := ilms.Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)

	attributes := dp.Attributes()
	issuer, _ := attributes.Get("tlscheck.x509.issuer")
	commonName, _ := attributes.Get("tlscheck.x509.cn")

	assert.Equal(t, "CN=ExpiredIssuer", issuer.AsString())
	assert.Equal(t, "expired.com", commonName.AsString())

	// Ensure that timeLeft is negative for an expired cert
	timeLeft := dp.IntValue()
	assert.Negative(t, timeLeft, int64(0), "Time left should be negative for an expired certificate")
}

func TestScrape_NotYetValidCertificate(t *testing.T) {
	cfg := &Config{
		Targets: []*confignet.TCPAddrConfig{
			{Endpoint: "expired.com:443"},
		},
	}
	settings := receivertest.NewNopSettings()
	s := newScraper(cfg, settings, mockGetConnectionStateNotYetValid)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, metrics.DataPointCount())

	rm := metrics.ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics().At(0)
	metric := ilms.Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)

	attributes := dp.Attributes()
	issuer, _ := attributes.Get("tlscheck.x509.issuer")
	commonName, _ := attributes.Get("tlscheck.x509.cn")

	assert.Equal(t, "CN=NotYetValidIssuer", issuer.AsString())
	assert.Equal(t, "notyetvalid.com", commonName.AsString())

	// Ensure that timeLeft is positive for a not-yet-valid cert
	timeLeft := dp.IntValue()
	assert.Positive(t, timeLeft, "Time left should be positive for a not-yet-valid cert")
}


*/
