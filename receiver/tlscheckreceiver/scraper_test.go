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
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

type MockTLSConn struct {
	net.Conn
	state tls.ConnectionState
}

func (m *MockTLSConn) ConnectionState() tls.ConnectionState {
	return m.state
}

func mockTLSDial(network, address string, config *tls.Config, validity string) (*MockTLSConn, error) {
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

	return &MockTLSConn{
		state: tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
		},
	}, nil
}

var tlsDial = func(network, addr string, config *tls.Config) (*MockTLSConn, error) {
	mockConn, err := mockTLSDial(network, addr, config, "valid") // default to "valid"
	return (*MockTLSConn)(mockConn), err
}

// func TestScrape_ValidCertificate(t *testing.T) {
// 	originalDial := tlsDial
// 	defer func() { tlsDial = originalDial }()
// 	tlsDial = func(network, addr string, config *tls.Config) (*MockTLSConn, error) {
// 		return mockTLSDial(network, addr, config, "valid")
// 	}

// 	cfg := &Config{
// 		Targets: []*targetConfig{
// 			{Host: "valid.com:443"},
// 		},
// 	}
// 	settings := receivertest.NewNopSettings()
// 	s := newScraper(cfg, settings)
// 	s.mb = metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)

// 	metrics, err := s.scrape(context.Background())
// 	require.NoError(t, err)
// 	assert.Equal(t, 1, metrics.DataPointCount())

// 	rm := metrics.ResourceMetrics().At(0)
// 	ilms := rm.ScopeMetrics().At(0)
// 	metric := ilms.Metrics().At(0)
// 	dp := metric.Gauge().DataPoints().At(0)

// 	timeLeft := dp.IntValue()
// 	assert.Greater(t, timeLeft, int64(0), "Time left should be positive for a valid certificate")
// }

func TestScrape_ExpiredCertificate(t *testing.T) {
	originalDial := tlsDial
	defer func() { tlsDial = originalDial }()
	tlsDial = func(network, addr string, config *tls.Config) (*MockTLSConn, error) {
		return mockTLSDial(network, addr, config, "expired")
	}

	cfg := &Config{
		Targets: []*targetConfig{
			{Host: "expired.com:9999"},
		},
	}
	settings := receivertest.NewNopSettings()
	s := newScraper(cfg, settings)
	s.mb = metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.DataPointCount())

	rm := metrics.ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics().At(0)
	metric := ilms.Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)

	timeLeft := dp.IntValue()
	assert.Less(t, timeLeft, int64(0), "Time left should be negative for an expired certificate")
}

func TestScrape_NotYetValidCertificate(t *testing.T) {
	originalDial := tlsDial
	defer func() { tlsDial = originalDial }()
	tlsDial = func(network, addr string, config *tls.Config) (*MockTLSConn, error) {
		return mockTLSDial(network, addr, config, "notYetValid")
	}

	cfg := &Config{
		Targets: []*targetConfig{
			{Host: "notyetvalid.com:8080"},
		},
	}
	settings := receivertest.NewNopSettings()
	s := newScraper(cfg, settings)
	s.mb = metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.DataPointCount())

	rm := metrics.ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics().At(0)
	metric := ilms.Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)

	timeLeft := dp.IntValue()
	assert.Greater(t, timeLeft, int64(0), "Time left should be positive for a certificate not yet valid")
}

func TestScrape_NoTargets(t *testing.T) {
	cfg := &Config{Targets: []*targetConfig{}}
	s := newScraper(cfg, receivertest.NewNopSettings())
	metrics, err := s.scrape(context.Background())
	assert.Error(t, err)
	assert.Equal(t, ErrMissingTargets, err)
	assert.Equal(t, 0, metrics.DataPointCount())
}

func TestScrape_InvalidHost(t *testing.T) {
	cfg := &Config{
		Targets: []*targetConfig{
			{Host: "invalid:1234"},
		},
	}
	s := newScraper(cfg, receivertest.NewNopSettings())
	metrics, err := s.scrape(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.DataPointCount())
}


