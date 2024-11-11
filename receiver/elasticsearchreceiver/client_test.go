// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchreceiver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"
)

func TestCreateClientInvalidEndpoint(t *testing.T) {
	_, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://\x00",
		},
	}, componenttest.NewNopHost())
	require.Error(t, err)
}

func TestNodeStatsNoPassword(t *testing.T) {
	nodeJSON := readSamplePayload(t, "nodes_stats_linux.json")

	actualNodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJSON, &actualNodeStats))

	elasticsearchMock := newMockServer(t)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)
	ctx := context.Background()
	nodeStats, err := client.NodeStats(ctx, []string{"_all"})
	require.NoError(t, err)

	require.Equal(t, &actualNodeStats, nodeStats)
}

func TestNodeStatsNilNodes(t *testing.T) {
	nodeJSON := readSamplePayload(t, "nodes_stats_linux.json")

	actualNodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJSON, &actualNodeStats))

	elasticsearchMock := newMockServer(t)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	nodeStats, err := client.NodeStats(ctx, nil)
	require.NoError(t, err)

	require.Equal(t, &actualNodeStats, nodeStats)
}

func TestNodeStatsNilIOStats(t *testing.T) {
	nodeJSON := readSamplePayload(t, "nodes_stats_other.json")

	actualNodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJSON, &actualNodeStats))

	elasticsearchMock := newMockServer(t, withNodes(nodeJSON))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	nodeStats, err := client.NodeStats(ctx, nil)
	require.NoError(t, err)

	require.Equal(t, &actualNodeStats, nodeStats)
}

func TestNodeStatsAuthentication(t *testing.T) {
	nodeJSON := readSamplePayload(t, "nodes_stats_linux.json")

	actualNodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJSON, &actualNodeStats))

	username := "user"
	password := "pass"

	elasticsearchMock := newMockServer(t, withBasicAuth(username, password))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
		Username: username,
		Password: configopaque.String(password),
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	nodeStats, err := client.NodeStats(ctx, []string{"_all"})
	require.NoError(t, err)

	require.Equal(t, &actualNodeStats, nodeStats)
}

func TestNodeStatsNoAuthentication(t *testing.T) {
	elasticsearchMock := newMockServer(t, withBasicAuth("user", "pass"))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.NodeStats(ctx, []string{"_all"})
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestNodeStatsBadAuthentication(t *testing.T) {
	elasticsearchMock := newMockServer(t, withBasicAuth("user", "pass"))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
		Username: "bad_user",
		Password: "bad_pass",
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.NodeStats(ctx, []string{"_all"})
	require.ErrorIs(t, err, errUnauthorized)
}

func TestClusterHealthNoPassword(t *testing.T) {
	healthJSON := readSamplePayload(t, "health.json")

	actualClusterHealth := model.ClusterHealth{}
	require.NoError(t, json.Unmarshal(healthJSON, &actualClusterHealth))

	elasticsearchMock := newMockServer(t)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	nodeStats, err := client.ClusterHealth(ctx)
	require.NoError(t, err)

	require.Equal(t, &actualClusterHealth, nodeStats)
}

func TestClusterHealthAuthentication(t *testing.T) {
	healthJSON := readSamplePayload(t, "health.json")

	actualClusterHealth := model.ClusterHealth{}
	require.NoError(t, json.Unmarshal(healthJSON, &actualClusterHealth))

	username := "user"
	password := "pass"

	elasticsearchMock := newMockServer(t, withBasicAuth(username, password))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
		Username: username,
		Password: configopaque.String(password),
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	nodeStats, err := client.ClusterHealth(ctx)
	require.NoError(t, err)

	require.Equal(t, &actualClusterHealth, nodeStats)
}

func TestClusterHealthNoAuthentication(t *testing.T) {
	elasticsearchMock := newMockServer(t, withBasicAuth("user", "pass"))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.ClusterHealth(ctx)
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestClusterHealthNoAuthorization(t *testing.T) {
	elasticsearchMock := newMockServer(t, withBasicAuth("user", "pass"))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
		Username: "bad_user",
		Password: "bad_pass",
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.ClusterHealth(ctx)
	require.ErrorIs(t, err, errUnauthorized)
}

func TestMetadataNoPassword(t *testing.T) {
	metadataJSON := readSamplePayload(t, "metadata.json")

	actualMetadata := model.ClusterMetadataResponse{}
	require.NoError(t, json.Unmarshal(metadataJSON, &actualMetadata))

	elasticsearchMock := newMockServer(t)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	metadata, err := client.ClusterMetadata(ctx)
	require.NoError(t, err)

	require.Equal(t, &actualMetadata, metadata)
}

func TestMetadataAuthentication(t *testing.T) {
	metadataJSON := readSamplePayload(t, "metadata.json")

	actualMetadata := model.ClusterMetadataResponse{}
	require.NoError(t, json.Unmarshal(metadataJSON, &actualMetadata))

	username := "user"
	password := "pass"

	elasticsearchMock := newMockServer(t, withBasicAuth(username, password))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
		Username: username,
		Password: configopaque.String(password),
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	metadata, err := client.ClusterMetadata(ctx)
	require.NoError(t, err)

	require.Equal(t, &actualMetadata, metadata)
}

func TestMetadataNoAuthentication(t *testing.T) {
	elasticsearchMock := newMockServer(t, withBasicAuth("user", "pass"))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.ClusterMetadata(ctx)
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestMetadataNoAuthorization(t *testing.T) {
	elasticsearchMock := newMockServer(t, withBasicAuth("user", "pass"))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
		Username: "bad_user",
		Password: "bad_pass",
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.ClusterMetadata(ctx)
	require.ErrorIs(t, err, errUnauthorized)
}

func TestDoRequestBadPath(t *testing.T) {
	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://example.localhost:9200",
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	_, err = client.doRequest(context.Background(), "\x7f")
	require.Error(t, err)
}

func TestDoRequestClientTimeout(t *testing.T) {
	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://example.localhost:9200",
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.doRequest(ctx, "_cluster/health")
	require.Error(t, err)
}

func TestDoRequest404(t *testing.T) {
	elasticsearchMock := newMockServer(t)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	_, err = client.doRequest(context.Background(), "invalid_path")
	require.ErrorContains(t, err, "404")
}

func TestIndexStatsNoPassword(t *testing.T) {
	indexJSON := readSamplePayload(t, "indices.json")

	actualIndexStats := model.IndexStats{}
	require.NoError(t, json.Unmarshal(indexJSON, &actualIndexStats))

	elasticsearchMock := newMockServer(t)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)
	ctx := context.Background()
	indexStats, err := client.IndexStats(ctx, []string{"_all"})
	require.NoError(t, err)

	require.Equal(t, &actualIndexStats, indexStats)
}

func TestIndexStatsNilNodes(t *testing.T) {
	indexJSON := readSamplePayload(t, "indices.json")

	actualIndexStats := model.IndexStats{}
	require.NoError(t, json.Unmarshal(indexJSON, &actualIndexStats))

	elasticsearchMock := newMockServer(t)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	indexStats, err := client.IndexStats(ctx, nil)
	require.NoError(t, err)

	require.Equal(t, &actualIndexStats, indexStats)
}

func TestIndexStatsAuthentication(t *testing.T) {
	indexJSON := readSamplePayload(t, "indices.json")

	actualIndexStats := model.IndexStats{}
	require.NoError(t, json.Unmarshal(indexJSON, &actualIndexStats))

	username := "user"
	password := "pass"

	elasticsearchMock := newMockServer(t, withBasicAuth(username, password))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
		Username: username,
		Password: configopaque.String(password),
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	indexStats, err := client.IndexStats(ctx, []string{"_all"})
	require.NoError(t, err)

	require.Equal(t, &actualIndexStats, indexStats)
}

func TestIndexStatsNoAuthentication(t *testing.T) {
	elasticsearchMock := newMockServer(t, withBasicAuth("user", "pass"))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.IndexStats(ctx, []string{"_all"})
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestIndexStatsBadAuthentication(t *testing.T) {
	elasticsearchMock := newMockServer(t, withBasicAuth("user", "pass"))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
		Username: "bad_user",
		Password: "bad_pass",
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.IndexStats(ctx, []string{"_all"})
	require.ErrorIs(t, err, errUnauthorized)
}

func TestClusterStatsNoPassword(t *testing.T) {
	clusterJSON := readSamplePayload(t, "cluster.json")

	actualClusterStats := model.ClusterStats{}
	require.NoError(t, json.Unmarshal(clusterJSON, &actualClusterStats))

	elasticsearchMock := newMockServer(t)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)
	ctx := context.Background()
	clusterStats, err := client.ClusterStats(ctx, []string{"_all"})
	require.NoError(t, err)

	require.Equal(t, &actualClusterStats, clusterStats)
}

func TestClusterStatsNilNodes(t *testing.T) {
	clusterJSON := readSamplePayload(t, "cluster.json")

	actualClusterStats := model.ClusterStats{}
	require.NoError(t, json.Unmarshal(clusterJSON, &actualClusterStats))

	elasticsearchMock := newMockServer(t)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	clusterStats, err := client.ClusterStats(ctx, nil)
	require.NoError(t, err)

	require.Equal(t, &actualClusterStats, clusterStats)
}

func TestClusterStatsAuthentication(t *testing.T) {
	clusterJSON := readSamplePayload(t, "cluster.json")

	actualClusterStats := model.ClusterStats{}
	require.NoError(t, json.Unmarshal(clusterJSON, &actualClusterStats))

	username := "user"
	password := "pass"

	elasticsearchMock := newMockServer(t, withBasicAuth(username, password))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
		Username: username,
		Password: configopaque.String(password),
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	clusterStats, err := client.ClusterStats(ctx, []string{"_all"})
	require.NoError(t, err)

	require.Equal(t, &actualClusterStats, clusterStats)
}

func TestClusterStatsNoAuthentication(t *testing.T) {
	elasticsearchMock := newMockServer(t, withBasicAuth("user", "pass"))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.ClusterStats(ctx, []string{"_all"})
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestClusterStatsBadAuthentication(t *testing.T) {
	elasticsearchMock := newMockServer(t, withBasicAuth("user", "pass"))
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(context.Background(), componenttest.NewNopTelemetrySettings(), Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: elasticsearchMock.URL,
		},
		Username: "bad_user",
		Password: "bad_pass",
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.ClusterStats(ctx, []string{"_all"})
	require.ErrorIs(t, err, errUnauthorized)
}

type mockServer struct {
	auth     func(username, password string) bool
	metadata []byte
	prefixes map[string][]byte
}

type mockServerOption func(*mockServer)

func withBasicAuth(username, password string) mockServerOption { // nolint:unparam
	return func(m *mockServer) {
		m.auth = func(u, p string) bool {
			return u == username && p == password
		}
	}
}

func withNodes(payload []byte) mockServerOption {
	return func(m *mockServer) {
		m.prefixes["/_nodes/_all/stats"] = payload
	}
}

// newMockServer gives a mock elasticsearch server for testing
func newMockServer(t *testing.T, opts ...mockServerOption) *httptest.Server {
	mock := mockServer{
		metadata: readSamplePayload(t, "metadata.json"),
		prefixes: map[string][]byte{
			"/_nodes/_all/stats": readSamplePayload(t, "nodes_stats_linux.json"),
			"/_all/_stats":       readSamplePayload(t, "indices.json"),
			"/_cluster/health":   readSamplePayload(t, "health.json"),
			"/_cluster/stats":    readSamplePayload(t, "cluster.json"),
		},
	}
	for _, opt := range opts {
		opt(&mock)
	}

	elasticsearchMock := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if mock.auth != nil {
			username, password, ok := req.BasicAuth()
			if !ok {
				rw.WriteHeader(http.StatusUnauthorized)
				return
			} else if !mock.auth(username, password) {
				rw.WriteHeader(http.StatusForbidden)
				return
			}
		}
		if req.URL.Path == "/" {
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write(mock.metadata)
			assert.NoError(t, err)
			return
		}
		for prefix, payload := range mock.prefixes {
			if strings.HasPrefix(req.URL.Path, prefix) {
				rw.WriteHeader(http.StatusOK)
				_, err := rw.Write(payload)
				assert.NoError(t, err)
				return
			}
		}
		rw.WriteHeader(http.StatusNotFound)
	}))

	return elasticsearchMock
}

func readSamplePayload(t *testing.T, file string) []byte {
	payload, err := os.ReadFile(filepath.Join("testdata", "sample_payloads", file))
	require.NoError(t, err)
	return payload
}
