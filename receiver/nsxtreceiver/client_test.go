// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nsxtreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver"

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
)

const (
	goodUser     = ""
	goodPassword = ""
	user500      = "user500"
	badPassword  = "password123"
)

func TestNewClientFailureToParse(t *testing.T) {
	_, err := newClient(&Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://\x00",
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.Error(t, err)
}

func TestTransportNodes(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)
	nodes, err := client.TransportNodes(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, nodes)
}

func TestClusterNodes(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)
	nodes, err := client.ClusterNodes(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, nodes)
}

func TestClusterNodeInterface(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)
	iFaces, err := client.Interfaces(context.Background(), managerNode1, managerClass)
	require.NoError(t, err)
	require.NotEmpty(t, iFaces)
}

func TestTransportNodeInterface(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)
	iFaces, err := client.Interfaces(context.Background(), transportNode1, transportClass)
	require.NoError(t, err)
	require.NotEmpty(t, iFaces)
}

func TestTransportNodeStatus(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)
	transportStatus, err := client.NodeStatus(context.Background(), transportNode1, transportClass)
	require.NoError(t, err)
	require.NotZero(t, transportStatus.SystemStatus.MemTotal)
}

func TestClusterNodeStatus(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)
	transportStatus, err := client.NodeStatus(context.Background(), managerNode1, managerClass)
	require.NoError(t, err)
	require.NotZero(t, transportStatus.SystemStatus.MemTotal)
}

func TestTransportNodeInterfaceStatus(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)
	iStats, err := client.InterfaceStatus(context.Background(), transportNode1, transportNodeNic1, transportClass)
	require.NoError(t, err)
	require.NotZero(t, iStats.RxBytes)
}

func TestManagerNodeInterfaceStatus(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)
	iStats, err := client.InterfaceStatus(context.Background(), managerNode1, managerNodeNic1, managerClass)
	require.NoError(t, err)
	require.NotZero(t, iStats.RxBytes)
}

func TestDoRequestBadUrl(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)

	_, err = client.doRequest(context.Background(), "\x00")
	require.ErrorContains(t, err, "parse")
}

func TestPermissionDenied_ClusterNodes(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		Password: badPassword,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)

	_, err = client.ClusterNodes(context.Background())
	require.ErrorContains(t, err, errUnauthorized.Error())
}

func TestPermissionDenied_Interfaces(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		Password: badPassword,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)

	_, err = client.Interfaces(context.Background(), managerNode1, managerClass)
	require.ErrorContains(t, err, errUnauthorized.Error())
}

func TestPermissionDenied_InterfaceStatus(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		Password: badPassword,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)

	_, err = client.InterfaceStatus(context.Background(), managerNode1, managerNodeNic1, managerClass)
	require.ErrorContains(t, err, errUnauthorized.Error())
}

func TestPermissionDenied_NodeStatus(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		Password: badPassword,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)

	_, err = client.NodeStatus(context.Background(), managerNode1, managerClass)
	require.ErrorContains(t, err, errUnauthorized.Error())
}

func TestPermissionDenied_TransportNodes(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		Password: badPassword,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)

	_, err = client.TransportNodes(context.Background())
	require.ErrorContains(t, err, errUnauthorized.Error())
}

func TestInternalServerError(t *testing.T) {
	nsxMock := mockServer(t)
	client, err := newClient(&Config{
		Username: user500,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nsxMock.URL,
		},
	}, componenttest.NewNopTelemetrySettings(), componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)

	_, err = client.ClusterNodes(context.Background())
	require.ErrorContains(t, err, "500")
}

// mockServer gives a mock NSX REST API server for testing; if username or password is included, they will be required for the client.
// otherwise, authorization is ignored.
func mockServer(t *testing.T) *httptest.Server {
	tNodeBytes, err := os.ReadFile(filepath.Join("testdata", "metrics", "transport_nodes.json"))
	require.NoError(t, err)

	cNodeBytes, err := os.ReadFile(filepath.Join("testdata", "metrics", "cluster_nodes.json"))
	require.NoError(t, err)

	mNodeInterfaces, err := os.ReadFile(filepath.Join("testdata", "metrics", "nodes", "cluster", managerNode1, "interfaces", "index.json"))
	require.NoError(t, err)

	tNodeInterfaces, err := os.ReadFile(filepath.Join("testdata", "metrics", "nodes", "transport", transportNode1, "interfaces", "index.json"))
	require.NoError(t, err)

	tNodeStatus, err := os.ReadFile(filepath.Join("testdata", "metrics", "nodes", "transport", transportNode1, "status.json"))
	require.NoError(t, err)

	mNodeStatus, err := os.ReadFile(filepath.Join("testdata", "metrics", "nodes", "cluster", managerNode1, "status.json"))
	require.NoError(t, err)

	tNodeInterfaceStats, err := os.ReadFile(filepath.Join("testdata", "metrics", "nodes", "transport", transportNode1, "interfaces", transportNodeNic1, "stats.json"))
	require.NoError(t, err)

	mNodeInterfaceStats, err := os.ReadFile(filepath.Join("testdata", "metrics", "nodes", "cluster", managerNode1, "interfaces", managerNodeNic1, "stats.json"))
	require.NoError(t, err)

	nsxMock := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		authUser, authPass, ok := req.BasicAuth()
		switch {
		case !ok:
			rw.WriteHeader(401)
			return
		case authUser == user500:
			rw.WriteHeader(500)
			return
		case authUser != goodUser || authPass != goodPassword:
			rw.WriteHeader(403)
			return
		}

		if req.URL.Path == "/api/v1/transport-nodes" {
			rw.WriteHeader(200)
			_, err = rw.Write(tNodeBytes)
			require.NoError(t, err)
			return
		}

		if req.URL.Path == "/api/v1/cluster/nodes" {
			rw.WriteHeader(200)
			_, err = rw.Write(cNodeBytes)
			require.NoError(t, err)
			return
		}

		if req.URL.Path == fmt.Sprintf("/api/v1/cluster/nodes/%s/network/interfaces", managerNode1) {
			rw.WriteHeader(200)
			_, err = rw.Write(mNodeInterfaces)
			require.NoError(t, err)
			return
		}

		if req.URL.Path == fmt.Sprintf("/api/v1/transport-nodes/%s/status", transportNode1) {
			rw.WriteHeader(200)
			_, err = rw.Write(tNodeStatus)
			require.NoError(t, err)
			return
		}

		if req.URL.Path == fmt.Sprintf("/api/v1/transport-nodes/%s/network/interfaces", transportNode1) {
			rw.WriteHeader(200)
			_, err = rw.Write(tNodeInterfaces)
			require.NoError(t, err)
			return
		}

		if req.URL.Path == fmt.Sprintf("/api/v1/transport-nodes/%s/network/interfaces/%s/stats", transportNode1, transportNodeNic1) {
			rw.WriteHeader(200)
			_, err = rw.Write(tNodeInterfaceStats)
			require.NoError(t, err)
			return
		}

		if req.URL.Path == fmt.Sprintf("/api/v1/cluster/nodes/%s/network/interfaces/%s/stats", managerNode1, managerNodeNic1) {
			rw.WriteHeader(200)
			_, err = rw.Write(mNodeInterfaceStats)
			require.NoError(t, err)
			return
		}

		if req.URL.Path == fmt.Sprintf("/api/v1/cluster/nodes/%s/status", managerNode1) {
			rw.WriteHeader(200)
			_, err = rw.Write(mNodeStatus)
			require.NoError(t, err)
			return
		}

		rw.WriteHeader(404)
	}))

	return nsxMock
}
