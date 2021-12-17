package model

// ClusterHealth represents a response from elasticsearch's /_cluster/health endpoint.
// The struct is not exhaustive; It does not provide all values returned by elasticsearch,
// only the ones relevant to the metrics retrieved by the scraper.
type ClusterHealth struct {
	ClusterName        string `json:"cluster_name"`
	ActiveShards       int64  `json:"active_shards"`
	RelocatingShards   int64  `json:"relocating_shards"`
	InitializingShards int64  `json:"initializing_shards"`
	UnassignedShards   int64  `json:"unassigned_shards"`
	NodeCount          int64  `json:"number_of_nodes"`
	DataNodeCount      int64  `json:"number_of_data_nodes"`
	Status             string `json:"status"`
}
