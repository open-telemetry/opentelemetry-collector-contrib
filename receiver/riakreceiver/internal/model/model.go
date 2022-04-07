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
