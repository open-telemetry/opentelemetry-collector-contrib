package elasticsearchreceiver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"
	"go.uber.org/zap"
)

var (
	errUnauthenticated = errors.New("status 401, unauthenticated")
	errUnauthorized    = errors.New("status 403, unauthorized")
)

// elasticsearchClient defines the interface to retreive metrics from an Elasticsearch cluster.
type elasticsearchClient interface {
	NodeStats(ctx context.Context) (*model.NodeStats, error)
	ClusterHealth(ctx context.Context) (*model.ClusterHealth, error)
}

// defaultElasticsearchClient is the main implementation of elastisearchClient.
// It retrieves the required metrics from Elasticsearch's REST api.
type defaultElasticsearchClient struct {
	client     *http.Client
	endpoint   *url.URL
	authHeader string
	logger     *zap.Logger
}

var _ elasticsearchClient = (*defaultElasticsearchClient)(nil)

func newElasticsearchClient(logger *zap.Logger, client *http.Client, endpoint *url.URL, username, password string) *defaultElasticsearchClient {
	var authHeader string
	if username != "" && password != "" {
		userPass := fmt.Sprintf("%s:%s", username, password)
		authb64 := base64.StdEncoding.EncodeToString([]byte(userPass))
		authHeader = fmt.Sprintf("Basic %s", authb64)
	}

	return &defaultElasticsearchClient{
		client:     client,
		authHeader: authHeader,
		endpoint:   endpoint,
		logger:     logger,
	}
}

// nodeStatsMetrics is a comma separated list of metrics that will be gathered from NodeStats.
// The available metrics are documented here for elasticsearch 7.9:
// https://www.elastic.co/guide/en/elasticsearch/reference/7.9/cluster-nodes-stats.html#cluster-nodes-stats-api-path-params
const nodeStatsMetrics = "indices,process,jvm,thread_pool,transport,http,fs"

// nodeStatsIndexMetrics is a comma separated list of index metrics that will be gathered from NodeStats.
const nodeStatsIndexMetrics = "store,docs,indexing,get,search,merge,refresh,flush,warmer,query_cache,fielddata"

func (c defaultElasticsearchClient) NodeStats(ctx context.Context) (*model.NodeStats, error) {
	nodeStatsPath := fmt.Sprintf("_nodes/stats/%s/%s", nodeStatsMetrics, nodeStatsIndexMetrics)

	body, err := c.doRequest(ctx, nodeStatsPath)
	if err != nil {
		return nil, err
	}

	nodeStats := model.NodeStats{}
	err = json.Unmarshal(body, &nodeStats)
	return &nodeStats, err
}

func (c defaultElasticsearchClient) ClusterHealth(ctx context.Context) (*model.ClusterHealth, error) {
	body, err := c.doRequest(ctx, "_cluster/health")
	if err != nil {
		return nil, err
	}

	clusterHealth := model.ClusterHealth{}
	err = json.Unmarshal(body, &clusterHealth)
	return &clusterHealth, err
}

func (c defaultElasticsearchClient) doRequest(ctx context.Context, path string) ([]byte, error) {
	endpoint, err := c.endpoint.Parse(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	if c.authHeader != "" {
		req.Header.Add("Authorization", c.authHeader)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		c.logger.Debug(
			"Request returned bad status code; Couldn't read request body.",
			zap.String("path", path),
			zap.Int("status_code", resp.StatusCode),
			zap.ByteString("body", body),
			zap.NamedError("body_read_error", err),
		)
	}

	switch resp.StatusCode {
	case 200: // OK
	case 401:
		return nil, errUnauthenticated
	case 403:
		return nil, errUnauthorized
	default:
		return nil, fmt.Errorf("got non 200 status code %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
