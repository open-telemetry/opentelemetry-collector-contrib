// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver/internal/model"

// Stat represents a stat in the API response
type Stats struct {
	// Identifiers
	Node string `json:"nodename"`

	// Metrics
	NodeGets           int64 `json:"node_gets"`
	NodePuts           int64 `json:"node_puts"`
	NodeGetFsmTimeMean int64 `json:"node_get_fsm_time_mean"`
	NodePutFsmTimeMean int64 `json:"node_put_fsm_time_mean"`
	ReadRepairs        int64 `json:"read_repairs_total"`
	MemAllocated       int64 `json:"mem_allocated"`
	VnodeGets          int64 `json:"vnode_gets"`
	VnodePuts          int64 `json:"vnode_puts"`
	VnodeIndexDeletes  int64 `json:"vnode_index_deletes"`
	VnodeIndexReads    int64 `json:"vnode_index_reads"`
	VnodeIndexWrites   int64 `json:"vnode_index_writes"`
}
