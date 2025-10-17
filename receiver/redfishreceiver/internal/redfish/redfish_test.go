package redfish

import (
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

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
	compSysId := "S7"

	ts := createRedfishTestServer(HPE, compSysId)
	defer ts.Close()

	client, err := NewClient(compSysId, ts.URL, USER, PASS, WithInsecure(true))
	require.NoError(t, err)

	system, err := client.GetComputerSystem()
	require.NoError(t, err)
	require.Equal(t, system.HostName, "otel-test-host")

	chassis, err := client.GetChassis(system.Links.Chassis[0].Ref)
	require.NoError(t, err)
	require.Equal(t, chassis.SerialNumber, "2M131992H0")

	thermal, err := client.GetThermal(chassis.Thermal.Ref)
	require.NoError(t, err)

	require.NotNil(t, thermal.Fans[0].Reading)
	require.Equal(t, *thermal.Fans[0].Reading, int64(50))

	require.NotNil(t, thermal.Temperatures[0].ReadingCelsius)
	require.Equal(t, *thermal.Temperatures[0].ReadingCelsius, int64(70))
}

// Test_Dell is a function to test all Dell redfish endpoints
func Test_Dell(t *testing.T) {
	compSysId := "System.Embedded.1"
	ts := createRedfishTestServer(DELL, compSysId)
	defer ts.Close()

	client, err := NewClient(compSysId, ts.URL, USER, PASS, WithInsecure(true))
	require.NoError(t, err)

	system, err := client.GetComputerSystem()
	require.NoError(t, err)
	require.Equal(t, system.HostName, "test-dell")

	chassis, err := client.GetChassis(system.Links.Chassis[0].Ref)
	require.NoError(t, err)
	require.Equal(t, chassis.SerialNumber, "MX777")

	thermal, err := client.GetThermal(chassis.Thermal.Ref)
	require.NoError(t, err)

	require.NotNil(t, thermal.Fans[0].Reading)
	require.Equal(t, *thermal.Fans[0].Reading, int64(7091))

	require.Nil(t, thermal.Temperatures[0].ReadingCelsius)
	require.NotNil(t, thermal.Temperatures[1].ReadingCelsius)
	require.Equal(t, *thermal.Temperatures[1].ReadingCelsius, int64(28))
}
