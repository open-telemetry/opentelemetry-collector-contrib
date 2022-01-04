package elasticsearchreceiver

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNodeStatsNoPassword(t *testing.T) {
	nodeJSON, err := ioutil.ReadFile("./testdata/sample_payloads/nodes_linux.json")
	require.NoError(t, err)

	actualNodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJSON, &actualNodeStats))

	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	url, err := url.Parse(elasticsearchMock.URL)
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, "", "")
	ctx := context.Background()
	nodeStats, err := client.NodeStats(ctx, []string{"_all"})
	require.NoError(t, err)

	require.Equal(t, &actualNodeStats, nodeStats)
}

func TestNodeStatsNilNodes(t *testing.T) {
	nodeJSON, err := ioutil.ReadFile("./testdata/sample_payloads/nodes_linux.json")
	require.NoError(t, err)

	actualNodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJSON, &actualNodeStats))

	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	url, err := url.Parse(elasticsearchMock.URL)
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, "", "")
	ctx := context.Background()
	nodeStats, err := client.NodeStats(ctx, nil)
	require.NoError(t, err)

	require.Equal(t, &actualNodeStats, nodeStats)
}

func TestNodeStatsAuthentication(t *testing.T) {
	nodeJSON, err := ioutil.ReadFile("./testdata/sample_payloads/nodes_linux.json")
	require.NoError(t, err)

	actualNodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJSON, &actualNodeStats))

	username := "user"
	password := "pass"

	elasticsearchMock := mockServer(t, username, password)
	defer elasticsearchMock.Close()

	url, err := url.Parse(elasticsearchMock.URL)
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, username, password)
	ctx := context.Background()
	nodeStats, err := client.NodeStats(ctx, []string{"_all"})
	require.NoError(t, err)

	require.Equal(t, &actualNodeStats, nodeStats)
}

func TestNodeStatsNoAuthentication(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	url, err := url.Parse(elasticsearchMock.URL)
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, "", "")
	ctx := context.Background()
	_, err = client.NodeStats(ctx, []string{"_all"})
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestNodeStatsBadAuthentication(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	url, err := url.Parse(elasticsearchMock.URL)
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, "unauthorized_user", "password")
	ctx := context.Background()
	_, err = client.NodeStats(ctx, []string{"_all"})
	require.ErrorIs(t, err, errUnauthorized)
}

func TestClusterHealthNoPassword(t *testing.T) {
	healthJSON, err := ioutil.ReadFile("./testdata/sample_payloads/health.json")
	require.NoError(t, err)

	actualClusterHealth := model.ClusterHealth{}
	require.NoError(t, json.Unmarshal(healthJSON, &actualClusterHealth))

	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	url, err := url.Parse(elasticsearchMock.URL)
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, "", "")
	ctx := context.Background()
	nodeStats, err := client.ClusterHealth(ctx)
	require.NoError(t, err)

	require.Equal(t, &actualClusterHealth, nodeStats)
}

func TestClusterHealthAuthentication(t *testing.T) {
	healthJSON, err := ioutil.ReadFile("./testdata/sample_payloads/health.json")
	require.NoError(t, err)

	actualClusterHealth := model.ClusterHealth{}
	require.NoError(t, json.Unmarshal(healthJSON, &actualClusterHealth))

	username := "user"
	password := "pass"

	elasticsearchMock := mockServer(t, username, password)
	defer elasticsearchMock.Close()

	url, err := url.Parse(elasticsearchMock.URL)
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, username, password)
	ctx := context.Background()
	nodeStats, err := client.ClusterHealth(ctx)
	require.NoError(t, err)

	require.Equal(t, &actualClusterHealth, nodeStats)
}

func TestClusterHealthNoAuthentication(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	url, err := url.Parse(elasticsearchMock.URL)
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, "", "")
	ctx := context.Background()
	_, err = client.ClusterHealth(ctx)
	require.ErrorIs(t, err, errUnauthenticated)
}

func TestClusterHealthNoAuthorization(t *testing.T) {
	elasticsearchMock := mockServer(t, "user", "pass")
	defer elasticsearchMock.Close()

	url, err := url.Parse(elasticsearchMock.URL)
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, "unauthorized_user", "password")
	ctx := context.Background()
	_, err = client.ClusterHealth(ctx)
	require.ErrorIs(t, err, errUnauthorized)
}

func TestDoRequestBadPath(t *testing.T) {
	url, err := url.Parse("http://example.localhost:9200")
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, "bad_username", "bad_password")
	_, err = client.doRequest(context.Background(), "\x7f")
	require.Error(t, err)
}

func TestDoRequestClientTimeout(t *testing.T) {
	url, err := url.Parse("http://example.localhost:9200")
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, "bad_username", "bad_password")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.doRequest(ctx, "_cluster/health")
	require.Error(t, err)
}

func TestDoRequest404(t *testing.T) {
	elasticsearchMock := mockServer(t, "", "")
	defer elasticsearchMock.Close()

	url, err := url.Parse(elasticsearchMock.URL)
	require.NoError(t, err)

	client := newElasticsearchClient(zap.NewNop(), http.DefaultClient, url, "", "")

	_, err = client.doRequest(context.Background(), "invalid_path")
	require.Error(t, err)
	require.Contains(t, err.Error(), "404")
}

// mockServer gives a mock elasticsearch server for testing; if username or password is included, they will be required for the client.
// otherwise, authorization is ignored.
func mockServer(t *testing.T, username, password string) *httptest.Server {
	nodes, err := ioutil.ReadFile("./testdata/sample_payloads/nodes_linux.json")
	require.NoError(t, err)
	health, err := ioutil.ReadFile("./testdata/sample_payloads/health.json")
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

		if strings.HasPrefix(req.URL.Path, "/_cluster/health") {
			rw.WriteHeader(200)
			_, err = rw.Write(health)
			require.NoError(t, err)
			return
		}
		rw.WriteHeader(404)
	}))

	return elasticsearchMock
}
