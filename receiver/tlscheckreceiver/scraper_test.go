package tlscheckreceiver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
)

type MockTLSConn struct {
	net.Conn
	state tls.ConnectionState
}

func (m *MockTLSConn) ConnectionState() tls.ConnectionState {
	return m.state
}

func mockTLSDial(network, address string, config *tls.Config, validity string) (*tls.Conn, error) {
	var cert *x509.Certificate
	now := time.Now()
	switch validity {
	case "valid":
		cert = &x509.Certificate{
			NotBefore: now.Add(-1 * time.Hour),
			NotAfter:  now.Add(24 * time.Hour),
			Subject:   pkix.Name{CommonName: "valid.com"},
			Issuer:    pkix.Name{CommonName: "ValidIssuer"},
		}
	case "expired":
		cert = &x509.Certificate{
			NotBefore: now.Add(-48 * time.Hour),
			NotAfter:  now.Add(-24 * time.Hour),
			Subject:   pkix.Name{CommonName: "expired.com"},
			Issuer:    pkix.Name{CommonName: "ExpiredIssuer"},
		}
	case "notYetValid":
		cert = &x509.Certificate{
			NotBefore: now.Add(24 * time.Hour),
			NotAfter:  now.Add(48 * time.Hour),
			Subject:   pkix.Name{CommonName: "notyetvalid.com"},
			Issuer:    pkix.Name{CommonName: "FutureIssuer"},
		}
	}

	mockConn := &MockTLSConn{
		state: tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
		},
	}
	return (*tls.Conn)(mockConn), nil
}

func TestScrape_ValidCertificate(t *testing.T) {
	originalDial := tlsDial
	defer func() { tlsDial = originalDial }()
	tlsDial = func(network, addr string, config *tls.Config) (*tls.Conn, error) {
		return mockTLSDial(network, addr, config, "valid")
	}

	cfg := &Config{Targets: []string{"valid.com:443"}}
	settings := receivertest.NewNopCreateSettings()
	s := newScraper(cfg, settings)
	s.mb = metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings.TelemetrySettings)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.DataPointCount())

	rm := metrics.ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics().At(0)
	metric := ilms.Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)

	timeLeft := dp.IntVal()
	assert.Greater(t, timeLeft, int64(0), "Time left should be positive for a valid certificate")
}

func TestScrape_ExpiredCertificate(t *testing.T) {
	originalDial := tlsDial
	defer func() { tlsDial = originalDial }()
	tlsDial = func(network, addr string, config *tls.Config) (*tls.Conn, error) {
		return mockTLSDial(network, addr, config, "expired")
	}

	cfg := &Config{Targets: []string{"expired.com:9999"}}
	settings := receivertest.NewNopCreateSettings()
	s := newScraper(cfg, settings)
	s.mb = metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings.TelemetrySettings)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.DataPointCount())

	rm := metrics.ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics().At(0)
	metric := ilms.Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)

	timeLeft := dp.IntVal()
	assert.Less(t, timeLeft, int64(0), "Time left should be negative for an expired certificate")
}

func TestScrape_NotYetValidCertificate(t *testing.T) {
	originalDial := tlsDial
	defer func() { tlsDial = originalDial }()
	tlsDial = func(network, addr string, config *tls.Config) (*tls.Conn, error) {
		return mockTLSDial(network, addr, config, "notYetValid")
	}

	cfg := &Config{Targets: []string{"notyetvalid.com:8080"}}
	settings := receivertest.NewNopCreateSettings()
	s := newScraper(cfg, settings)
	s.mb = metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings.TelemetrySettings)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.DataPointCount())

	rm := metrics.ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics().At(0)
	metric := ilms.Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)

	timeLeft := dp.IntVal()
	assert.Greater(t, timeLeft, int64(0), "Time left should be positive for a certificate not yet valid")
}

func TestScrape_NoTargets(t *testing.T) {
	cfg := &Config{Targets: []string{}}
	s := newScraper(cfg, receivertest.NewNopCreateSettings())
	metrics, err := s.scrape(context.Background())
	assert.Error(t, err)
	assert.Equal(t, errMissingHost, err)
	assert.Equal(t, 0, metrics.DataPointCount())
}

func TestScrape_InvalidHost(t *testing.T) {
	cfg := &Config{Targets: []string{"invalid:1234"}}
	s := newScraper(cfg, receivertest.NewNopCreateSettings())
	metrics, err := s.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.DataPointCount())
}


