// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package simpleprometheusreceiver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/internal/metadata"
)

// TestGetPrometheusConfigInlineTLS proves that inline PEM TLS material and the
// server name are forwarded to the underlying prometheus receiver. Both are
// valid confighttp.ClientConfig TLS options; dropping them silently turns a
// TLS scrape into a misconfigured one with no error.
func TestGetPrometheusConfigInlineTLS(t *testing.T) {
	caPem, err := os.ReadFile("./testdata/test_cert.pem")
	require.NoError(t, err)

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "localhost:1234"
	clientConfig.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAPem: configopaque.String(caPem),
		},
		ServerName: "example.com",
	}

	cfg := &Config{
		ClientConfig:       clientConfig,
		CollectionInterval: 10 * time.Second,
		MetricsPath:        "/metrics",
	}

	got, err := getPrometheusConfig(cfg)
	require.NoError(t, err)

	tlsCfg := got.PrometheusConfig.ScrapeConfigs[0].HTTPClientConfig.TLSConfig
	assert.Equal(t, string(caPem), tlsCfg.CA, "inline CA PEM (ca_pem) must be forwarded to the prometheus scrape config")
	assert.Equal(t, "example.com", tlsCfg.ServerName, "server_name must be forwarded to the prometheus scrape config")
}

// TestStartInlineTLS runs the forwarded TLS settings through the real
// receiver Start path, which re-marshals the prometheus config. Inspecting
// getPrometheusConfig output alone does not catch failures on that reload.
func TestStartInlineTLS(t *testing.T) {
	caPem, err := os.ReadFile("./testdata/test_cert.pem")
	require.NoError(t, err)
	certPem, keyPem := generateSelfSignedPEM(t)

	tests := []struct {
		name string
		tls  configtls.ClientConfig
	}{
		{
			name: "ca_pem",
			tls: configtls.ClientConfig{
				Config: configtls.Config{CAPem: configopaque.String(caPem)},
			},
		},
		{
			name: "server_name_override",
			tls: configtls.ClientConfig{
				ServerName: "example.com",
			},
		},
		{
			name: "cert_pem and key_pem",
			tls: configtls.ClientConfig{
				Config: configtls.Config{
					CertPem: configopaque.String(certPem),
					KeyPem:  configopaque.String(keyPem),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.ClientConfig.Endpoint = "localhost:1234"
			cfg.ClientConfig.TLS = tt.tls

			r, err := NewFactory().CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			require.NoError(t, err)
			require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
			require.NoError(t, r.Shutdown(t.Context()))
		})
	}
}

func generateSelfSignedPEM(t *testing.T) (certPem, keyPem []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDer, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certPem = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPem = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer})
	return certPem, keyPem
}
