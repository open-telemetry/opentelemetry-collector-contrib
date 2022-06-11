// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	NodeStats(ctx context.Context, nodes []string) (*model.NodeStats, error)
	ClusterHealth(ctx context.Context) (*model.ClusterHealth, error)
}

// defaultElasticsearchClient is the main implementation of elasticsearchClient.
// It retrieves the required metrics from Elasticsearch's REST api.
type defaultElasticsearchClient struct {
	client     *http.Client
	endpoint   *url.URL
	authHeader string
	logger     *zap.Logger
}

var _ elasticsearchClient = (*defaultElasticsearchClient)(nil)

func newElasticsearchClient(settings component.TelemetrySettings, c Config, h component.Host) (*defaultElasticsearchClient, error) {
	client, err := c.HTTPClientSettings.ToClient(h.GetExtensions(), settings)
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, err
	}

	var authHeader string
	if c.Username != "" && c.Password != "" {
		userPass := fmt.Sprintf("%s:%s", c.Username, c.Password)
		authb64 := base64.StdEncoding.EncodeToString([]byte(userPass))
		authHeader = fmt.Sprintf("Basic %s", authb64)
	}

	return &defaultElasticsearchClient{
		client:     client,
		authHeader: authHeader,
		endpoint:   endpoint,
		logger:     settings.Logger,
	}, nil
}

// nodeStatsMetrics is a comma separated list of metrics that will be gathered from NodeStats.
// The available metrics are documented here for Elasticsearch 7.9:
// https://www.elastic.co/guide/en/elasticsearch/reference/7.9/cluster-nodes-stats.html#cluster-nodes-stats-api-path-params
const nodeStatsMetrics = "indices,process,jvm,thread_pool,transport,http,fs"

// nodeStatsIndexMetrics is a comma separated list of index metrics that will be gathered from NodeStats.
const nodeStatsIndexMetrics = "store,docs,indexing,get,search,merge,refresh,flush,warmer,query_cache,fielddata"

func (c defaultElasticsearchClient) NodeStats(ctx context.Context, nodes []string) (*model.NodeStats, error) {
	var nodeSpec string
	if len(nodes) > 0 {
		nodeSpec = strings.Join(nodes, ",")
	} else {
		nodeSpec = "_all"
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
