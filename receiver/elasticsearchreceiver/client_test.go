// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchreceiver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"
)

func TestCreateClientInvalidEndpoint(t *testing.T) {
	_, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://\x00",
		},
	}, componenttest.NewNopHost())
	require.Error(t, err)
}

func TestNodeStatsNoPassword(t *testing.T) {
	nodeJSON, err := os.ReadFile("./testdata/sample_payloads/nodes_stats_linux.json")
	require.NoError(t, err)

	actualNodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJSON, &actualNodeStats))

	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	nodeJSON, err := os.ReadFile("./testdata/sample_payloads/nodes_stats_linux.json")
	require.NoError(t, err)

	actualNodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJSON, &actualNodeStats))

	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	nodeJSON, err := os.ReadFile("./testdata/sample_payloads/nodes_stats_linux.json")
	require.NoError(t, err)

	actualNodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJSON, &actualNodeStats))

	username := "user"
	password := "pass"

	elasticsearchMock := mockServer(t, username, password)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: elasticsearchMock.URL,
		},
		Username: username,
		Password: password,
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	nodeStats, err := client.NodeStats(ctx, []string{"_all"})
	require.NoError(t, err)

	require.Equal(t, &actualNodeStats, nodeStats)
}

func TestNodeStatsNoAuthentication(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.NodeStats(ctx, []string{"_all"})
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestNodeStatsBadAuthentication(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	healthJSON, err := os.ReadFile("./testdata/sample_payloads/health.json")
	require.NoError(t, err)

	actualClusterHealth := model.ClusterHealth{}
	require.NoError(t, json.Unmarshal(healthJSON, &actualClusterHealth))

	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	healthJSON, err := os.ReadFile("./testdata/sample_payloads/health.json")
	require.NoError(t, err)

	actualClusterHealth := model.ClusterHealth{}
	require.NoError(t, json.Unmarshal(healthJSON, &actualClusterHealth))

	username := "user"
	password := "pass"

	elasticsearchMock := mockServer(t, username, password)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: elasticsearchMock.URL,
		},
		Username: username,
		Password: password,
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	nodeStats, err := client.ClusterHealth(ctx)
	require.NoError(t, err)

	require.Equal(t, &actualClusterHealth, nodeStats)
}

func TestClusterHealthNoAuthentication(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.ClusterHealth(ctx)
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestClusterHealthNoAuthorization(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	metadataJSON, err := os.ReadFile("./testdata/sample_payloads/metadata.json")
	require.NoError(t, err)

	actualMetadata := model.ClusterMetadataResponse{}
	require.NoError(t, json.Unmarshal(metadataJSON, &actualMetadata))

	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	metadataJSON, err := os.ReadFile("./testdata/sample_payloads/metadata.json")
	require.NoError(t, err)

	actualMetadata := model.ClusterMetadataResponse{}
	require.NoError(t, json.Unmarshal(metadataJSON, &actualMetadata))

	username := "user"
	password := "pass"

	elasticsearchMock := mockServer(t, username, password)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: elasticsearchMock.URL,
		},
		Username: username,
		Password: password,
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	metadata, err := client.ClusterMetadata(ctx)
	require.NoError(t, err)

	require.Equal(t, &actualMetadata, metadata)
}

func TestMetadataNoAuthentication(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.ClusterMetadata(ctx)
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestMetadataNoAuthorization(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://example.localhost:9200",
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	_, err = client.doRequest(context.Background(), "\x7f")
	require.Error(t, err)
}

func TestDoRequestClientTimeout(t *testing.T) {
	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	_, err = client.doRequest(context.Background(), "invalid_path")
	require.Error(t, err)
	require.Contains(t, err.Error(), "404")
}

func TestIndexStatsNoPassword(t *testing.T) {
	indexJSON, err := os.ReadFile("./testdata/sample_payloads/indices.json")
	require.NoError(t, err)

	actualIndexStats := model.IndexStats{}
	require.NoError(t, json.Unmarshal(indexJSON, &actualIndexStats))

	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	indexJSON, err := os.ReadFile("./testdata/sample_payloads/indices.json")
	require.NoError(t, err)

	actualIndexStats := model.IndexStats{}
	require.NoError(t, json.Unmarshal(indexJSON, &actualIndexStats))

	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	indexJSON, err := os.ReadFile("./testdata/sample_payloads/indices.json")
	require.NoError(t, err)

	actualIndexStats := model.IndexStats{}
	require.NoError(t, json.Unmarshal(indexJSON, &actualIndexStats))

	username := "user"
	password := "pass"

	elasticsearchMock := mockServer(t, username, password)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: elasticsearchMock.URL,
		},
		Username: username,
		Password: password,
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	indexStats, err := client.IndexStats(ctx, []string{"_all"})
	require.NoError(t, err)

	require.Equal(t, &actualIndexStats, indexStats)
}

func TestIndexStatsNoAuthentication(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.IndexStats(ctx, []string{"_all"})
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestIndexStatsBadAuthentication(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	clusterJSON, err := os.ReadFile("./testdata/sample_payloads/cluster.json")
	require.NoError(t, err)

	actualClusterStats := model.ClusterStats{}
	require.NoError(t, json.Unmarshal(clusterJSON, &actualClusterStats))

	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	clusterJSON, err := os.ReadFile("./testdata/sample_payloads/cluster.json")
	require.NoError(t, err)

	actualClusterStats := model.ClusterStats{}
	require.NoError(t, json.Unmarshal(clusterJSON, &actualClusterStats))

	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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
	clusterJSON, err := os.ReadFile("./testdata/sample_payloads/cluster.json")
	require.NoError(t, err)

	actualClusterStats := model.ClusterStats{}
	require.NoError(t, json.Unmarshal(clusterJSON, &actualClusterStats))

	username := "user"
	password := "pass"

	elasticsearchMock := mockServer(t, username, password)
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: elasticsearchMock.URL,
		},
		Username: username,
		Password: password,
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	clusterStats, err := client.ClusterStats(ctx, []string{"_all"})
	require.NoError(t, err)

	require.Equal(t, &actualClusterStats, clusterStats)
}

func TestClusterStatsNoAuthentication(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: elasticsearchMock.URL,
		},
	}, componenttest.NewNopHost())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.ClusterStats(ctx, []string{"_all"})
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestClusterStatsBadAuthentication(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
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

// mockServer gives a mock elasticsearch server for testing; if username or password is included, they will be required for the client.
// otherwise, authorization is ignored.
func mockServer(t *testing.T, username, password string) *httptest.Server {
	nodes, err := os.ReadFile("./testdata/sample_payloads/nodes_stats_linux.json")
	require.NoError(t, err)
	indices, err := os.ReadFile("./testdata/sample_payloads/indices.json")
	require.NoError(t, err)
	health, err := os.ReadFile("./testdata/sample_payloads/health.json")
	require.NoError(t, err)
	metadata, err := os.ReadFile("./testdata/sample_payloads/metadata.json")
	require.NoError(t, err)
	cluster, err := os.ReadFile("./testdata/sample_payloads/cluster.json")
	require.NoError(t, err)

	elasticsearchMock := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if username != "" || password != "" {
			authUser, authPass, ok := req.BasicAuth()
			if !ok {
				rw.WriteHeader(401)
				return
			} else if authUser != username || authPass != password {
				rw.WriteHeader(403)
				return
			}
		}

		if strings.HasPrefix(req.URL.Path, "/_nodes/_all/stats") {
			rw.WriteHeader(200)
			_, err = rw.Write(nodes)
			require.NoError(t, err)
			return
		}

		if strings.HasPrefix(req.URL.Path, "/_all/_stats") {
			rw.WriteHeader(200)
			_, err = rw.Write(indices)
			require.NoError(t, err)
			return
		}

		if strings.HasPrefix(req.URL.Path, "/_cluster/health") {
			rw.WriteHeader(200)
			_, err = rw.Write(health)
			require.NoError(t, err)
			return
		}

		if strings.HasPrefix(req.URL.Path, "/_cluster/stats") {
			rw.WriteHeader(200)
			_, err = rw.Write(cluster)
			require.NoError(t, err)
			return
		}

		// metadata check
		if req.URL.Path == "/" {
			rw.WriteHeader(200)
			_, err = rw.Write(metadata)
			require.NoError(t, err)
			return
		}
		rw.WriteHeader(404)
	}))

	return elasticsearchMock
}
