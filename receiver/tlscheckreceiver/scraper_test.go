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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

//nolint:revive
func mockGetConnectionStateValid(endpoint string) (tls.ConnectionState, error) {
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
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	factory := receivertest.NewNopFactory()
	settings := receivertest.NewNopSettings(factory.Type())
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
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	factory := receivertest.NewNopFactory()
	settings := receivertest.NewNopSettings(factory.Type())
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
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	factory := receivertest.NewNopFactory()
	settings := receivertest.NewNopSettings(factory.Type())
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

func TestScrape_MultipleEndpoints(t *testing.T) {
	cfg := &Config{
		Targets: []*confignet.TCPAddrConfig{
			{Endpoint: "example1.com:443"},
			{Endpoint: "example2.com:443"},
			{Endpoint: "example3.com:443"},
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	factory := receivertest.NewNopFactory()
	settings := receivertest.NewNopSettings(factory.Type())
	s := newScraper(cfg, settings, mockGetConnectionStateValid)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)

	// Verify we have metrics for all endpoints
	assert.Equal(t, 3, metrics.ResourceMetrics().Len(), "Should have metrics for all endpoints")

	// Create a map of endpoints to their expected metrics
	expectedMetrics := map[string]struct {
		issuer     string
		commonName string
	}{
		"example1.com:443": {
			issuer:     "CN=ValidIssuer",
			commonName: "valid.com",
		},
		"example2.com:443": {
			issuer:     "CN=ValidIssuer",
			commonName: "valid.com",
		},
		"example3.com:443": {
			issuer:     "CN=ValidIssuer",
			commonName: "valid.com",
		},
	}

	// Check each resource metric
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)

		// Get the endpoint resource attribute
		endpoint, exists := rm.Resource().Attributes().Get("tlscheck.endpoint")
		require.True(t, exists, "Resource should have tlscheck.endpoint attribute")

		endpointStr := endpoint.AsString()
		expected, ok := expectedMetrics[endpointStr]
		require.True(t, ok, "Unexpected endpoint found: %s", endpointStr)

		// Remove the endpoint from expected metrics as we've found it
		delete(expectedMetrics, endpointStr)

		// Verify we have the expected metrics for this endpoint
		ilms := rm.ScopeMetrics().At(0)
		metric := ilms.Metrics().At(0)
		dp := metric.Gauge().DataPoints().At(0)

		// Verify the metric attributes
		attributes := dp.Attributes()
		issuer, _ := attributes.Get("tlscheck.x509.issuer")
		commonName, _ := attributes.Get("tlscheck.x509.cn")

		assert.Equal(t, expected.issuer, issuer.AsString(), "Incorrect issuer for endpoint %s", endpointStr)
		assert.Equal(t, expected.commonName, commonName.AsString(), "Incorrect common name for endpoint %s", endpointStr)
	}

	// Verify we found all expected endpoints
	assert.Empty(t, expectedMetrics, "All expected endpoints should have been found")
}
