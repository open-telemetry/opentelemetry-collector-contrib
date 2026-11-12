// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
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

	client, err := NewClient(compSysID, ts.URL, USER, PASS)
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

	client, err := NewClient(compSysID, ts.URL, USER, PASS)
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

// Test_BasicAuth is a function to test that basic auth is only sent when a username is set
func Test_BasicAuth(t *testing.T) {
	testCases := []struct {
		desc     string
		user     string
		pwd      string
		wantAuth bool
	}{
		{desc: "No Username", wantAuth: false},
		{desc: "With Username", user: USER, pwd: PASS, wantAuth: true},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, pass, ok := r.BasicAuth()
				if ok != tc.wantAuth || user != tc.user || pass != tc.pwd {
					http.Error(w, "unexpected basic auth", http.StatusUnauthorized)
					return
				}
				http.ServeFile(w, r, path.Join("testdata/", HPE, "systems.json"))
			}))
			defer ts.Close()

			client, err := NewClient("S7", ts.URL, tc.user, configopaque.String(tc.pwd))
			require.NoError(t, err)
			_, err = client.GetComputerSystem()
			require.NoError(t, err)
		})
	}
}

// Test_WithTLSConfig is a function to test that the TLS config is used by the HTTP transport
func Test_WithTLSConfig(t *testing.T) {
	tlsConfig := &tls.Config{ServerName: "bmc.example.com"}

	client, err := NewClient("S7", "https://localhost:3000", "", "", WithTLSConfig(tlsConfig))
	require.NoError(t, err)

	transport, ok := client.client.Transport.(*http.Transport)
	require.True(t, ok)
	require.Same(t, tlsConfig, transport.TLSClientConfig)
}
