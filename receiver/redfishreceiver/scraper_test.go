// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver/internal/metadata"
)

// newClientCertPEM is a helper function to create a self-signed client certificate and key in PEM format
func newClientCertPEM(t *testing.T) (certPEM, keyPEM []byte, cert *x509.Certificate) {
	return newCertPEM(t, x509.ExtKeyUsageClientAuth)
}

// newCertPEM is a helper function to create a self-signed certificate and key in PEM format
func newCertPEM(t *testing.T, usage x509.ExtKeyUsage, dnsNames ...string) (certPEM, keyPEM []byte, cert *x509.Certificate) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "otel-redfish-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		ExtKeyUsage:           []x509.ExtKeyUsage{usage},
		DNSNames:              dnsNames,
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err = x509.ParseCertificate(der)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, cert
}

func newTestScraper(servers ...Server) *redfishScraper {
	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		Servers:              servers,
	}
	return newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
}

var scraperTestResources = []Resource{ComputerSystemResource}

func TestScraperStart(t *testing.T) {
	testCases := []struct {
		desc        string
		server      Server
		expectError bool
	}{
		{
			desc:   "No TLS",
			server: Server{BaseURL: "https://localhost:3000", User: "user", Pwd: "pwd", ComputerSystemID: "1", Resources: scraperTestResources},
		},
		{
			desc: "Missing CA File",
			server: Server{
				BaseURL:          "https://localhost:3000",
				TLS:              configtls.ClientConfig{Config: configtls.Config{CAFile: filepath.Join(t.TempDir(), "missing.crt")}},
				ComputerSystemID: "1",
				Resources:        scraperTestResources,
			},
			expectError: true,
		},
		{
			desc: "Missing Client Cert File",
			server: Server{
				BaseURL: "https://localhost:3000",
				TLS: configtls.ClientConfig{Config: configtls.Config{
					CertFile: filepath.Join(t.TempDir(), "missing.crt"),
					KeyFile:  filepath.Join(t.TempDir(), "missing.key"),
				}},
				ComputerSystemID: "1",
				Resources:        scraperTestResources,
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			s := newTestScraper(tc.server)
			err := s.start(t.Context(), nil)
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, s.clients, 1)
		})
	}
}

func TestScraperStartMTLS(t *testing.T) {
	certPEM, keyPEM, clientCert := newClientCertPEM(t)
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(clientCert)

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := r.BasicAuth(); ok {
			http.Error(w, "unexpected basic auth", http.StatusBadRequest)
			return
		}
		http.ServeFile(w, r, filepath.Join("internal", "redfish", "testdata", "hpe", "systems.json"))
	}))
	ts.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
	ts.StartTLS()
	defer ts.Close()

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})

	s := newTestScraper(Server{
		BaseURL: ts.URL,
		TLS: configtls.ClientConfig{Config: configtls.Config{
			CAPem:   configopaque.String(caPEM),
			CertPem: configopaque.String(certPEM),
			KeyPem:  configopaque.String(keyPEM),
		}},
		ComputerSystemID: "1",
		Resources:        scraperTestResources,
	})
	require.NoError(t, s.start(t.Context(), nil))
	require.Len(t, s.clients, 1)

	system, err := s.clients[0].GetComputerSystem()
	require.NoError(t, err)
	require.Equal(t, "otel-test-host", system.HostName)
}

func TestScraperStartMTLSSinglePEMWithServerNameOverride(t *testing.T) {
	const serverName = "bmc.example.internal"

	clientCertPEM, clientKeyPEM, clientCert := newClientCertPEM(t)
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(clientCert)

	// the server certificate is only valid for serverName, not for the 127.0.0.1 listener address
	serverCertPEM, serverKeyPEM, _ := newCertPEM(t, x509.ExtKeyUsageServerAuth, serverName)
	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	require.NoError(t, err)

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("internal", "redfish", "testdata", "hpe", "systems.json"))
	}))
	ts.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
	}
	ts.StartTLS()
	defer ts.Close()

	// client certificate and key in a single PEM file, used for both cert_file and key_file
	dir := t.TempDir()
	clientPEMFile := filepath.Join(dir, "client.pem")
	require.NoError(t, os.WriteFile(clientPEMFile, append(clientCertPEM, clientKeyPEM...), 0o600))
	caFile := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(caFile, serverCertPEM, 0o600))

	newServer := func(serverNameOverride string) Server {
		return Server{
			BaseURL: ts.URL,
			TLS: configtls.ClientConfig{
				Config: configtls.Config{
					CAFile:   caFile,
					CertFile: clientPEMFile,
					KeyFile:  clientPEMFile,
				},
				ServerName: serverNameOverride,
			},
			ComputerSystemID: "1",
			Resources:        scraperTestResources,
		}
	}

	// without server_name_override the server certificate does not match the host
	s := newTestScraper(newServer(""))
	require.NoError(t, s.start(t.Context(), nil))
	_, err = s.clients[0].GetComputerSystem()
	require.Error(t, err)

	s = newTestScraper(newServer(serverName))
	require.NoError(t, s.start(t.Context(), nil))
	system, err := s.clients[0].GetComputerSystem()
	require.NoError(t, err)
	require.Equal(t, "otel-test-host", system.HostName)
}
