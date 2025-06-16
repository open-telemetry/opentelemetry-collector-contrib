// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
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
		DNSNames:  []string{"foo.example.com", "bar.example.com", "*.example.com"},
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
		NotBefore: time.Now().Add(24 * time.Hour),
		NotAfter:  time.Now().Add(48 * time.Hour),
		Subject:   pkix.Name{CommonName: "notyetvalid.com"},
		Issuer:    pkix.Name{CommonName: "NotYetValidIssuer"},
	}
	return tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}, nil
}

func createMockCertFile(t *testing.T, expiry time.Time) string {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// issuer cert
	issuerTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(123456789),
		Subject:      pkix.Name{CommonName: "FooIssuer"},
		NotBefore:    time.Now(),
		NotAfter:     expiry,
		IsCA:         true,
	}
	issuerCertBytes, err := x509.CreateCertificate(rand.Reader, issuerTemplate, issuerTemplate, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)
	issuerCert, err := x509.ParseCertificate(issuerCertBytes)
	require.NoError(t, err)

	// leaf cert
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(123456789),
		NotBefore:    time.Now(),
		NotAfter:     expiry,
		Subject:      pkix.Name{CommonName: "test.example.com"},
		Issuer:       pkix.Name{CommonName: "FooIssuer"},
	}
	leafCertBytes, err := x509.CreateCertificate(rand.Reader, leafTemplate, issuerTemplate, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)
	leafCert, err := x509.ParseCertificate(leafCertBytes)
	require.NoError(t, err)
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-cert-*.pem")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	for _, cert := range []*x509.Certificate{leafCert, issuerCert} {
		err = pem.Encode(tmpFile, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
		require.NoError(t, err)
	}

	err = tmpFile.Close()
	require.NoError(t, err)

	return tmpFile.Name()
}

func TestScrape_ExpiredEndpointCertificate(t *testing.T) {
	cfg := &Config{
		Targets: []*CertificateTarget{
			{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "expired.com:443",
				},
			},
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
	assert.Negative(t, timeLeft, "Time left should be negative for an expired certificate")
}

func TestScrape_NotYetValidEndpointCertificate(t *testing.T) {
	cfg := &Config{
		Targets: []*CertificateTarget{
			{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "expired.com:443",
				},
			},
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
		Targets: []*CertificateTarget{
			{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "example1.com:443",
				},
			},
			{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "example2.com:443",
				},
			},
			{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "example3.com:443",
				},
			},
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

		// Get the target resource attribute
		target, exists := rm.Resource().Attributes().Get("tlscheck.target")
		require.True(t, exists, "Resource should have tlscheck.target attribute")

		targetStr := target.AsString()
		expected, ok := expectedMetrics[targetStr]
		require.True(t, ok, "Unexpected target found: %s", targetStr)

		// Remove the target from expected metrics as we've found it
		delete(expectedMetrics, targetStr)

		// Verify we have the expected metrics for this target
		ilms := rm.ScopeMetrics().At(0)
		metric := ilms.Metrics().At(0)
		dp := metric.Gauge().DataPoints().At(0)

		// Verify the metric attributes
		attributes := dp.Attributes()
		issuer, _ := attributes.Get("tlscheck.x509.issuer")
		commonName, _ := attributes.Get("tlscheck.x509.cn")

		assert.Equal(t, expected.issuer, issuer.AsString(), "Incorrect issuer for target %s", targetStr)
		assert.Equal(t, expected.commonName, commonName.AsString(), "Incorrect common name for target %s", targetStr)
	}

	// Verify we found all expected endpoints
	assert.Empty(t, expectedMetrics, "All expected endpoints should have been found")
}

func TestScrape_ExpiredFilepathCertificate(t *testing.T) {
	caCertFile := createMockCertFile(t, time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC))
	cfg := &Config{
		Targets: []*CertificateTarget{
			{
				FilePath: caCertFile,
			},
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	factory := receivertest.NewNopFactory()
	settings := receivertest.NewNopSettings(factory.Type())
	s := newScraper(cfg, settings, mockGetConnectionStateValid)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())

	rm := metrics.ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics().At(0)
	metric := ilms.Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)
	target, exists := rm.Resource().Attributes().Get("tlscheck.target")
	require.True(t, exists)
	assert.Equal(t, caCertFile, target.AsString())

	// Verify negative time left on cert
	timeLeft := dp.IntValue()
	assert.Negative(t, timeLeft, "Time left should be negative for an expired cert")
}

func TestScrape_ValidFilepathCertificate(t *testing.T) {
	caCertFile := createMockCertFile(t, time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))
	cfg := &Config{
		Targets: []*CertificateTarget{
			{
				FilePath: caCertFile,
			},
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	factory := receivertest.NewNopFactory()
	settings := receivertest.NewNopSettings(factory.Type())
	s := newScraper(cfg, settings, mockGetConnectionStateValid)

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())

	rm := metrics.ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics().At(0)
	metric := ilms.Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)
	target, exists := rm.Resource().Attributes().Get("tlscheck.target")
	require.True(t, exists)
	assert.Equal(t, caCertFile, target.AsString())

	// Verify the metric attributes
	attributes := dp.Attributes()
	issuer, _ := attributes.Get("tlscheck.x509.issuer")
	commonName, _ := attributes.Get("tlscheck.x509.cn")
	assert.Equal(t, "CN=FooIssuer", issuer.AsString(), "Incorrect issuer for target %s", caCertFile)
	assert.Equal(t, "test.example.com", commonName.AsString(), "Incorrect common name for target %s", caCertFile)

	// Verify positive time left on cert
	timeLeft := dp.IntValue()
	assert.Positive(t, timeLeft, "Time left should be positive for a valid cert")
}

func TestValidateEndpoint(t *testing.T) {
	testCases := []struct {
		desc        string
		endpoint    string
		expectedErr string
	}{
		{
			desc:        "valid endpoint",
			endpoint:    "example.com:443",
			expectedErr: "",
		},
		{
			desc:        "invalid endpoint - bad port",
			endpoint:    "bad-endpoint:12efg",
			expectedErr: "provided port is not a number: 12efg",
		},
		{
			desc:        "endpoint with scheme",
			endpoint:    "https://example.com:443",
			expectedErr: "endpoint contains a scheme, which is not allowed",
		},
		{
			desc:        "port out of range",
			endpoint:    "example.com:67000",
			expectedErr: "provided port is out of valid range [1, 65535]: 67000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validateEndpoint(tc.endpoint)
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateFilepath(t *testing.T) {
	// Create a temporary certificate file for testing
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-cert-*.pem")
	require.NoError(t, err)
	tmpFile.Close()

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	testCases := []struct {
		desc        string
		filePath    string
		expectedErr string
	}{
		{
			desc:        "valid file path",
			filePath:    tmpFile.Name(),
			expectedErr: "",
		},
		{
			desc:        "relative file path",
			filePath:    "cert.pem",
			expectedErr: "error accessing certificate file",
		},
		{
			desc:        "nonexistent file",
			filePath:    "D:/nonexistent/path/cert.pem",
			expectedErr: "error accessing certificate file",
		},
		{
			desc:        "directory instead of file",
			filePath:    tmpDir,
			expectedErr: "path is a directory, not a file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validateFilepath(tc.filePath)
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
