// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"

// ClusterStats represents a response from elasticsearch's /_cluster/stats endpoint.
// The struct is not exhaustive; It does not provide all values returned by elasticsearch,
// only the ones relevant to the metrics retrieved by the scraper.
// The structures don't match the ones used in nodes and index responses.
type ClusterStats struct {
	ClusterName  string              `json:"cluster_name"`
	IndicesStats ClusterIndicesStats `json:"indices"`
	NodesStats   ClusterNodesStats   `json:"nodes"`
}

type ClusterIndicesStats struct {
	FieldDataCache ClusterCacheInfo `json:"fielddata"`
	QueryCache     ClusterCacheInfo `json:"query_cache"`
}

type ClusterCacheInfo struct {
	Evictions int64 `json:"evictions"`
}

type ClusterNodesStats struct {
	JVMInfo ClusterNodesJVMInfo `json:"jvm"`
}

type ClusterNodesJVMInfo struct {
	JVMMemoryInfo ClusterNodesJVMMemInfo `json:"mem"`
}

type ClusterNodesJVMMemInfo struct {
	HeapUsedInBy int64 `json:"heap_used_in_bytes"`
}
