// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	USER = "test"
	PASS = "test"
	HPE  = "hpe"
	DELL = "dell"
)

// createRedfishTestServer is a helper function to create a test http redfish server
func createRedfishTestServer(oem, computerSystemID string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok && user == USER && pass == PASS {
			http.Error(w, "authentication failed", http.StatusUnauthorized)
			return
		}

		filePath := path.Join("testdata/", oem, "/")
		switch r.URL.Path {
		case path.Join("/redfish/v1/Chassis/", computerSystemID, "/Thermal"):
			http.ServeFile(w, r, path.Join(filePath, "thermal.json"))
		case path.Join("/redfish/v1/Systems/", computerSystemID):
			http.ServeFile(w, r, path.Join(filePath, "systems.json"))
		case path.Join("/redfish/v1/Chassis/", computerSystemID):
			http.ServeFile(w, r, path.Join(filePath, "chassis.json"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// Test_HPE is a function to test all HPE redfish endpoints
func Test_HPE(t *testing.T) {
	compSysID := "S7"

	ts := createRedfishTestServer(HPE, compSysID)
	defer ts.Close()

	client, err := NewClient(compSysID, ts.URL, USER, PASS, WithInsecure(true))
	require.NoError(t, err)

	system, err := client.GetComputerSystem()
	require.NoError(t, err)
	require.Equal(t, "otel-test-host", system.HostName)

	chassis, err := client.GetChassis(system.Links.Chassis[0].Ref)
	require.NoError(t, err)
	require.Equal(t, "2M131992H0", chassis.SerialNumber)

	thermal, err := client.GetThermal(chassis.Thermal.Ref)
	require.NoError(t, err)

	require.NotNil(t, thermal.Fans[0].Reading)
	require.Equal(t, int64(50), *thermal.Fans[0].Reading)

	require.NotNil(t, thermal.Temperatures[0].ReadingCelsius)
	require.Equal(t, int64(70), *thermal.Temperatures[0].ReadingCelsius)
}

// Test_Dell is a function to test all Dell redfish endpoints
func Test_Dell(t *testing.T) {
	compSysID := "System.Embedded.1"
	ts := createRedfishTestServer(DELL, compSysID)
	defer ts.Close()

	client, err := NewClient(compSysID, ts.URL, USER, PASS, WithInsecure(true))
	require.NoError(t, err)

	system, err := client.GetComputerSystem()
	require.NoError(t, err)
	require.Equal(t, "test-dell", system.HostName)

	chassis, err := client.GetChassis(system.Links.Chassis[0].Ref)
	require.NoError(t, err)
	require.Equal(t, "MX777", chassis.SerialNumber)

	thermal, err := client.GetThermal(chassis.Thermal.Ref)
	require.NoError(t, err)

	require.NotNil(t, thermal.Fans[0].Reading)
	require.Equal(t, int64(7091), *thermal.Fans[0].Reading)

	require.Nil(t, thermal.Temperatures[0].ReadingCelsius)
	require.NotNil(t, thermal.Temperatures[1].ReadingCelsius)
	require.Equal(t, int64(28), *thermal.Temperatures[1].ReadingCelsius)
}

// newClientCert is a helper function to create a self-signed client certificate
func newClientCert(t *testing.T) (tls.Certificate, *x509.Certificate) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "otel-redfish-client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, cert
}

// Test_MTLS is a function to test client certificate authentication without basic auth
func Test_MTLS(t *testing.T) {
	compSysID := "S7"
	clientCert, caCert := newClientCert(t)
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(caCert)

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := r.BasicAuth(); ok {
			http.Error(w, "unexpected basic auth", http.StatusBadRequest)
			return
		}
		http.ServeFile(w, r, path.Join("testdata/", HPE, "systems.json"))
	}))
	ts.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
	ts.StartTLS()
	defer ts.Close()

	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(ts.Certificate())

	// without a client certificate the handshake must fail
	client, err := NewClient(compSysID, ts.URL, "", "", WithTLSConfig(&tls.Config{RootCAs: rootCAs}))
	require.NoError(t, err)
	_, err = client.GetComputerSystem()
	require.Error(t, err)

	client, err = NewClient(compSysID, ts.URL, "", "", WithTLSConfig(&tls.Config{
		RootCAs:      rootCAs,
		Certificates: []tls.Certificate{clientCert},
	}))
	require.NoError(t, err)
	system, err := client.GetComputerSystem()
	require.NoError(t, err)
	require.Equal(t, "otel-test-host", system.HostName)
}

// Test_MTLSWithBasicAuth is a function to test client certificate authentication combined with basic auth
func Test_MTLSWithBasicAuth(t *testing.T) {
	compSysID := "S7"
	clientCert, caCert := newClientCert(t)
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(caCert)

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != USER || pass != PASS {
			http.Error(w, "authentication failed", http.StatusUnauthorized)
			return
		}
		http.ServeFile(w, r, path.Join("testdata/", HPE, "systems.json"))
	}))
	ts.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
	ts.StartTLS()
	defer ts.Close()

	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(ts.Certificate())

	client, err := NewClient(compSysID, ts.URL, USER, PASS, WithTLSConfig(&tls.Config{
		RootCAs:      rootCAs,
		Certificates: []tls.Certificate{clientCert},
	}))
	require.NoError(t, err)
	system, err := client.GetComputerSystem()
	require.NoError(t, err)
	require.Equal(t, "otel-test-host", system.HostName)
}

// Test_InsecureWithTLSConfig is a function to test that insecure skips server verification when a TLS config is set
func Test_InsecureWithTLSConfig(t *testing.T) {
	compSysID := "S7"
	clientCert, caCert := newClientCert(t)
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(caCert)

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path.Join("testdata/", HPE, "systems.json"))
	}))
	ts.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
	ts.StartTLS()
	defer ts.Close()

	// the server certificate is not trusted, so verification must fail
	client, err := NewClient(compSysID, ts.URL, "", "", WithTLSConfig(&tls.Config{
		Certificates: []tls.Certificate{clientCert},
	}))
	require.NoError(t, err)
	_, err = client.GetComputerSystem()
	require.Error(t, err)

	client, err = NewClient(compSysID, ts.URL, "", "", WithInsecure(true), WithTLSConfig(&tls.Config{
		Certificates: []tls.Certificate{clientCert},
	}))
	require.NoError(t, err)
	system, err := client.GetComputerSystem()
	require.NoError(t, err)
	require.Equal(t, "otel-test-host", system.HostName)
}
