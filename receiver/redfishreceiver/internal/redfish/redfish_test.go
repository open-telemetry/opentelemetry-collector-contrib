package redfish

import (
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	USER               = "test"
	PASS               = "test"
	COMPUTER_SYSTEM_ID = "S7"
	HPE                = "hpe"
)

// createRedfishTestServer is a helper function to create a test http redfish server
func createRedfishTestServer(oem, computerSystemID string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok && user == USER && pass == PASS {
			http.Error(w, "authentication failed", http.StatusUnauthorized)
			return
		}

		switch oem {
		case HPE:
			switch r.URL.Path {
			case path.Join("/redfish/v1/Chassis/", computerSystemID, "/Thermal"):
				http.ServeFile(w, r, "testdata/hpe-thermal.json")
			case path.Join("/redfish/v1/Systems/", computerSystemID):
				http.ServeFile(w, r, "testdata/hpe-systems.json")
			case path.Join("/redfish/v1/Chassis/", computerSystemID):
				http.ServeFile(w, r, "testdata/hpe-chassis.json")
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// Test_HPE is a function to test all HPE redfish endpoints
func Test_HPE(t *testing.T) {
	ts := createRedfishTestServer(HPE, COMPUTER_SYSTEM_ID)
	defer ts.Close()

	client, err := NewClient(COMPUTER_SYSTEM_ID, ts.URL, USER, PASS, WithInsecure(true))
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
