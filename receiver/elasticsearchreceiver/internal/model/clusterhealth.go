// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"

// ClusterHealth represents a response from elasticsearch's /_cluster/health endpoint.
// The struct is not exhaustive; It does not provide all values returned by elasticsearch,
// only the ones relevant to the metrics retrieved by the scraper.
type ClusterHealth struct {
	ClusterName             string `json:"cluster_name"`
	ActiveShards            int64  `json:"active_shards"`
	ActivePrimaryShards     int64  `json:"active_primary_shards"`
	RelocatingShards        int64  `json:"relocating_shards"`
	InitializingShards      int64  `json:"initializing_shards"`
	UnassignedShards        int64  `json:"unassigned_shards"`
	DelayedUnassignedShards int64  `json:"delayed_unassigned_shards"`
	NodeCount               int64  `json:"number_of_nodes"`
	DataNodeCount           int64  `json:"number_of_data_nodes"`
	PendingTasksCount       int64  `json:"number_of_pending_tasks"`
	InFlightFetchCount      int64  `json:"number_of_in_flight_fetch"`
	Status                  string `json:"status"`
}
