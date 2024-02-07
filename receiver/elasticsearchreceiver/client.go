// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/go-version"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"
)

var (
	errUnauthenticated = errors.New("status 401, unauthenticated")
	errUnauthorized    = errors.New("status 403, unauthorized")
)

// elasticsearchClient defines the interface to retrieve metrics from an Elasticsearch cluster.
type elasticsearchClient interface {
	Nodes(ctx context.Context, nodes []string) (*model.Nodes, error)
	NodeStats(ctx context.Context, nodes []string) (*model.NodeStats, error)
	ClusterHealth(ctx context.Context) (*model.ClusterHealth, error)
	IndexStats(ctx context.Context, indices []string) (*model.IndexStats, error)
	ClusterMetadata(ctx context.Context) (*model.ClusterMetadataResponse, error)
	ClusterStats(ctx context.Context, nodes []string) (*model.ClusterStats, error)
}

// defaultElasticsearchClient is the main implementation of elasticsearchClient.
// It retrieves the required metrics from Elasticsearch's REST api.
type defaultElasticsearchClient struct {
	client     *http.Client
	endpoint   *url.URL
	authHeader string
	logger     *zap.Logger
	version    *version.Version
}

var _ elasticsearchClient = (*defaultElasticsearchClient)(nil)

func newElasticsearchClient(settings component.TelemetrySettings, c Config, h component.Host) (*defaultElasticsearchClient, error) {
	client, err := c.HTTPClientSettings.ToClient(h, settings)
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, err
	}

	var authHeader string
	if c.Username != "" && c.Password != "" {
		userPass := fmt.Sprintf("%s:%s", c.Username, string(c.Password))
		authb64 := base64.StdEncoding.EncodeToString([]byte(userPass))
		authHeader = fmt.Sprintf("Basic %s", authb64)
	}

	esClient := defaultElasticsearchClient{
		client:     client,
		authHeader: authHeader,
		endpoint:   endpoint,
		logger:     settings.Logger,
	}

	// Try update es version
	_, _ = esClient.ClusterMetadata(context.Background())
	return &esClient, nil
}

var (
	es7_9 = func() *version.Version {
		v, _ := version.NewVersion("7.9")
		return v
	}()
)

const (
	// A comma separated list of metrics that will be gathered from NodeStats.
	// https://www.elastic.co/guide/en/elasticsearch/reference/7.9/cluster-nodes-stats.html#cluster-nodes-stats-api-path-params
	defaultNodeStatsMetrics = "breaker,indices,process,jvm,thread_pool,transport,http,fs,ingest,indices,adaptive_selection,discovery,script,os"

	// Extra NodeStats Metrics that are only supported on and after 7.9
	nodeStatsMetricsAfter7_9 = ",indexing_pressure"

	// A comma separated list of metrics that will be gathered from Nodes.
	// The available metrics are documented here for Elasticsearch 7.9:
	// https://www.elastic.co/guide/en/elasticsearch/reference/7.9/cluster-nodes-info.html
	// Note: This constant should remain empty as the receiver will only retrieve metadata from the /_nodes endpoint, not metrics.
	nodesMetrics = ""

	// A comma separated list of index metrics that will be gathered from NodeStats.
	nodeStatsIndexMetrics = "store,docs,indexing,get,search,merge,refresh,flush,warmer,query_cache,fielddata,translog"

	// A comma separated list of metrics that will be gathered from IndexStats.
	indexStatsMetrics = "_all"
)

func (c defaultElasticsearchClient) Nodes(ctx context.Context, nodeIds []string) (*model.Nodes, error) {
	var nodeSpec string
	if len(nodeIds) > 0 {
		nodeSpec = strings.Join(nodeIds, ",")
	} else {
		nodeSpec = "_all"
	}

	nodesPath := fmt.Sprintf("_nodes/%s/%s", nodeSpec, nodesMetrics)

	body, err := c.doRequest(ctx, nodesPath)
	if err != nil {
		return nil, err
	}

	nodes := model.Nodes{}
	err = json.Unmarshal(body, &nodes)
	return &nodes, err
}

func (c defaultElasticsearchClient) NodeStats(ctx context.Context, nodes []string) (*model.NodeStats, error) {
	var nodeSpec string
	if len(nodes) > 0 {
		nodeSpec = strings.Join(nodes, ",")
	} else {
		nodeSpec = "_all"
	}

	nodeStatsMetrics := defaultNodeStatsMetrics
	if c.version != nil && c.version.GreaterThanOrEqual(es7_9) {
		nodeStatsMetrics += nodeStatsMetricsAfter7_9
	}
	nodeStatsPath := fmt.Sprintf("_nodes/%s/stats/%s/%s", nodeSpec, nodeStatsMetrics, nodeStatsIndexMetrics)

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

func (c defaultElasticsearchClient) IndexStats(ctx context.Context, indices []string) (*model.IndexStats, error) {
	var indexSpec string
	if len(indices) > 0 {
		indexSpec = strings.Join(indices, ",")
	} else {
		indexSpec = "_all"
	}

	indexStatsPath := fmt.Sprintf("%s/_stats/%s", indexSpec, indexStatsMetrics)

	body, err := c.doRequest(ctx, indexStatsPath)
	if err != nil {
		return nil, err
	}

	indexStats := model.IndexStats{}
	err = json.Unmarshal(body, &indexStats)

	return &indexStats, err
}

func (c *defaultElasticsearchClient) ClusterMetadata(ctx context.Context) (*model.ClusterMetadataResponse, error) {
	body, err := c.doRequest(ctx, "")
	if err != nil {
		return nil, err
	}

	versionResponse := model.ClusterMetadataResponse{}
	err = json.Unmarshal(body, &versionResponse)
	if c.version == nil {
		c.version, _ = version.NewVersion(versionResponse.Version.Number)
	}
	return &versionResponse, err
}

func (c defaultElasticsearchClient) ClusterStats(ctx context.Context, nodes []string) (*model.ClusterStats, error) {
	var nodesSpec string
	if len(nodes) > 0 {
		nodesSpec = strings.Join(nodes, ",")
	} else {
		nodesSpec = "_all"
	}

	clusterStatsPath := fmt.Sprintf("_cluster/stats/nodes/%s", nodesSpec)

	body, err := c.doRequest(ctx, clusterStatsPath)
	if err != nil {
		return nil, err
	}

	clusterStats := model.ClusterStats{}
	err = json.Unmarshal(body, &clusterStats)

	return &clusterStats, err
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

	// See https://www.elastic.co/guide/en/elasticsearch/reference/8.0/api-conventions.html#api-compatibility
	// the compatible-with=7 should signal to newer version of Elasticsearch to use the v7.x API format
	req.Header.Add("Accept", "application/vnd.elasticsearch+json; compatible-with=7")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return io.ReadAll(resp.Body)
	}

	body, err := io.ReadAll(resp.Body)
	c.logger.Debug(
		"Failed to make request to Elasticsearch",
		zap.String("path", path),
		zap.Int("status_code", resp.StatusCode),
		zap.ByteString("body", body),
		zap.NamedError("body_read_error", err),
	)

	switch resp.StatusCode {
	case 401:
		return nil, errUnauthenticated
	case 403:
		return nil, errUnauthorized
	default:
		return nil, fmt.Errorf("got non 200 status code %d", resp.StatusCode)
	}
}
