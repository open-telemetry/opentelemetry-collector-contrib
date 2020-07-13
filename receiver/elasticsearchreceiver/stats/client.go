package stats

import (
	"net/http"

	es "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/client"
)

const (
	nodeStatsEndpoint          = "_nodes/_local/stats/transport,http,process,jvm,indices,thread_pool"
	clusterHealthStatsEndpoint = "_cluster/health"
	nodeInfoEndpoint           = "_nodes/_local"
	masterNodeEndpoint         = "_cluster/state/master_node"
	allIndexStatsEndpoint      = "_all/_stats"
)

// ESStatsHTTPClient holds methods hitting various ES stats endpoints
type ESStatsHTTPClient struct {
	esClient *es.ESClient
}

// NewESClient creates a new esClient
func NewESClient(endpoint string, client *http.Client) ESStatsHTTPClient {
	return ESStatsHTTPClient{
		esClient: &es.ESClient{
			Endpoint:   endpoint,
			HTTPClient: client,
		},
	}
}

// Method to collect index stats
func (c *ESStatsHTTPClient) GetIndexStats() (*IndexStatsOutput, error) {
	var indexStatsOutput IndexStatsOutput

	err := c.esClient.FetchJSON(allIndexStatsEndpoint, &indexStatsOutput)

	return &indexStatsOutput, err
}

// Method to identify the master node
func (c *ESStatsHTTPClient) GetMasterNodeInfo() (*MasterInfoOutput, error) {
	var masterInfoOutput MasterInfoOutput

	err := c.esClient.FetchJSON(masterNodeEndpoint, &masterInfoOutput)

	return &masterInfoOutput, err
}

// Method to fetch node info
func (c *ESStatsHTTPClient) GetNodeInfo() (*NodeInfoOutput, error) {
	var nodeInfoOutput NodeInfoOutput

	err := c.esClient.FetchJSON(nodeInfoEndpoint, &nodeInfoOutput)

	return &nodeInfoOutput, err
}

// Method to fetch cluster stats
func (c *ESStatsHTTPClient) GetClusterStats() (*ClusterStatsOutput, error) {
	var clusterStatsOutput ClusterStatsOutput

	err := c.esClient.FetchJSON(clusterHealthStatsEndpoint, &clusterStatsOutput)

	return &clusterStatsOutput, err
}

// Method to fetch node stats
func (c *ESStatsHTTPClient) GetNodeAndThreadPoolStats() (*NodeStatsOutput, error) {
	var nodeStatsOutput NodeStatsOutput

	err := c.esClient.FetchJSON(nodeStatsEndpoint, &nodeStatsOutput)

	return &nodeStatsOutput, err
}
