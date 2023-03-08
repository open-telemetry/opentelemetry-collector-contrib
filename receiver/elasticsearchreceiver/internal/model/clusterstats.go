// Copyright The OpenTelemetry Authors
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
