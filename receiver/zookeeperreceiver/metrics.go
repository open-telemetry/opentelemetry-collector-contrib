// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zookeeperreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver"

import (
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver/internal/metadata"
)

// Constants to define entries in the output of "mntr" command.
const (
	avgLatencyMetricKey              = "zk_avg_latency"
	maxLatencyMetricKey              = "zk_max_latency"
	minLatencyMetricKey              = "zk_min_latency"
	packetsReceivedMetricKey         = "zk_packets_received"
	packetsSentMetricKey             = "zk_packets_sent"
	numAliveConnectionsMetricKey     = "zk_num_alive_connections"
	outstandingRequestsMetricKey     = "zk_outstanding_requests"
	zNodeCountMetricKey              = "zk_znode_count"
	watchCountMetricKey              = "zk_watch_count"
	ephemeralsCountMetricKey         = "zk_ephemerals_count"
	approximateDataSizeMetricKey     = "zk_approximate_data_size"
	openFileDescriptorCountMetricKey = "zk_open_file_descriptor_count"
	maxFileDescriptorCountMetricKey  = "zk_max_file_descriptor_count"

	fSyncThresholdExceedCountMetricKey = "zk_fsync_threshold_exceed_count"

	followersMetricKey       = "zk_followers"
	syncedFollowersMetricKey = "zk_synced_followers"
	pendingSyncsMetricKey    = "zk_pending_syncs"

	serverStateKey = "zk_server_state"
	zkVersionKey   = "zk_version"
)

func recordDataPointsFunc(mb *metadata.MetricsBuilder, metric string) func(ts pdata.Timestamp, val int64) {
	switch metric {
	case followersMetricKey:
		return mb.RecordZookeeperFollowersDataPoint
	case syncedFollowersMetricKey:
		return mb.RecordZookeeperSyncedFollowersDataPoint
	case pendingSyncsMetricKey:
		return mb.RecordZookeeperPendingSyncsDataPoint
	case avgLatencyMetricKey:
		return mb.RecordZookeeperLatencyAvgDataPoint
	case maxLatencyMetricKey:
		return mb.RecordZookeeperLatencyMaxDataPoint
	case minLatencyMetricKey:
		return mb.RecordZookeeperLatencyMinDataPoint
	case numAliveConnectionsMetricKey:
		return mb.RecordZookeeperConnectionsAliveDataPoint
	case outstandingRequestsMetricKey:
		return mb.RecordZookeeperOutstandingRequestsDataPoint
	case zNodeCountMetricKey:
		return mb.RecordZookeeperZnodesDataPoint
	case watchCountMetricKey:
		return mb.RecordZookeeperWatchesDataPoint
	case ephemeralsCountMetricKey:
		return mb.RecordZookeeperEphemeralNodesDataPoint
	case approximateDataSizeMetricKey:
		return mb.RecordZookeeperApproximateDateSizeDataPoint
	case openFileDescriptorCountMetricKey:
		return mb.RecordZookeeperOpenFileDescriptorsDataPoint
	case maxFileDescriptorCountMetricKey:
		return mb.RecordZookeeperMaxFileDescriptorsDataPoint
	case fSyncThresholdExceedCountMetricKey:
		return mb.RecordZookeeperFsyncThresholdExceedsDataPoint
	case packetsReceivedMetricKey:
		return mb.RecordZookeeperPacketsReceivedDataPoint
	case packetsSentMetricKey:
		return mb.RecordZookeeperPacketsSentDataPoint
	}

	return nil
}
