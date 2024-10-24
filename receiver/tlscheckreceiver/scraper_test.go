// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

//nolint:revive
func mockGetConnectionStateValid(host string) (tls.ConnectionState, error) {
	cert := &x509.Certificate{
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		Subject:   pkix.Name{CommonName: "valid.com"},
		Issuer:    pkix.Name{CommonName: "ValidIssuer"},
	}
	return tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}, nil
}

//nolint:revive
func mockGetConnectionStateExpired(host string) (tls.ConnectionState, error) {
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
func mockGetConnectionStateNotYetValid(host string) (tls.ConnectionState, error) {
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
