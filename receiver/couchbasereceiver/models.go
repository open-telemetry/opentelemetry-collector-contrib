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

package couchbasereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchbasereceiver"

// Cluster response models

type clusterResponse struct {
	StorageTotals storageTotals  `json:"storageTotals"`
	Nodes         []node         `json:"nodes"`
	BucketsInfo   clusterBuckets `json:"buckets"`
}

type storageTotals struct {
	RAM ramStorageTotals `json:"ram"`
	HDD hddStorageTotals `json:"hdd"`
}

type ramStorageTotals struct {
	Total             *int64 `json:"total"`
	QuotaTotal        *int64 `json:"quotaTotal"`
	QuotaUsed         *int64 `json:"quotaUsed"`
	Used              *int64 `json:"used"`
	UsedByData        *int64 `json:"usedByData"`
	QuotaUsedPerNode  *int64 `json:"quotaUsedPerNode"`
	QuotaTotalPerNode *int64 `json:"quotaTotalPerNode"`
}

type hddStorageTotals struct {
	Total      *int64 `json:"total"`
	QuotaTotal *int64 `json:"quotaTotal"`
	Used       *int64 `json:"used"`
	UsedByData *int64 `json:"usedByData"`
	Free       *int64 `json:"free"`
}

type clusterBuckets struct {
	URI string `json:"uri"`
}

// Node specific models

type node struct {
	Hostname         string               `json:"hostname"`
	InterestingStats nodeInterestingStats `json:"interestingStats"`
}

type nodeInterestingStats struct {
	ActiveItems      *int64 `json:"curr_items"`
	ReplicaItems     *int64 `json:"vb_replica_curr_items"`
	DocumentDataSize *int64 `json:"couch_docs_data_size"`
	DocumentDiskSize *int64 `json:"couch_docs_actual_disk_size"`
	ViewsDataSize    *int64 `json:"couch_views_data_size"`
	ViewsDiskSize    *int64 `json:"couch_views_actual_disk_size"`
}

// Bucket specific models

type bucket struct {
	Name      string          `json:"name"`
	StatsInfo bucketStatsInfo `json:"stats"`
}

type bucketStatsInfo struct {
	URI string `json:"uri"`
}

type bucketStats struct {
	Op bucketStatsOp `json:"op"`
}

type bucketStatsOp struct {
	LastTimeStamp int64                    `json:"lastTStamp"`
	Samples       map[string][]interface{} `json:"samples"`
}
