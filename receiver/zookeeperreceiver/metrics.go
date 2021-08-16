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

package zookeeperreceiver

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

func getOTLPInitFunc(metric string) func(metric pdata.Metric) {
	switch metric {
	case followersMetricKey:
		return metadata.Metrics.ZookeeperFollowers.Init
	case syncedFollowersMetricKey:
		return metadata.Metrics.ZookeeperSyncedFollowers.Init
	case pendingSyncsMetricKey:
		return metadata.Metrics.ZookeeperPendingSyncs.Init
	case avgLatencyMetricKey:
		return metadata.Metrics.ZookeeperLatencyAvg.Init
	case maxLatencyMetricKey:
		return metadata.Metrics.ZookeeperLatencyMax.Init
	case minLatencyMetricKey:
		return metadata.Metrics.ZookeeperLatencyMin.Init
	case numAliveConnectionsMetricKey:
		return metadata.Metrics.ZookeeperConnectionsAlive.Init
	case outstandingRequestsMetricKey:
		return metadata.Metrics.ZookeeperOutstandingRequests.Init
	case zNodeCountMetricKey:
		return metadata.Metrics.ZookeeperZnodes.Init
	case watchCountMetricKey:
		return metadata.Metrics.ZookeeperWatches.Init
	case ephemeralsCountMetricKey:
		return metadata.Metrics.ZookeeperEphemeralNodes.Init
	case approximateDataSizeMetricKey:
		return metadata.Metrics.ZookeeperApproximateDateSize.Init
	case openFileDescriptorCountMetricKey:
		return metadata.Metrics.ZookeeperOpenFileDescriptors.Init
	case maxFileDescriptorCountMetricKey:
		return metadata.Metrics.ZookeeperMaxFileDescriptors.Init
	case fSyncThresholdExceedCountMetricKey:
		return metadata.Metrics.ZookeeperFsyncThresholdExceeds.Init
	case packetsReceivedMetricKey:
		return metadata.Metrics.ZookeeperPacketsReceived.Init
	case packetsSentMetricKey:
		return metadata.Metrics.ZookeeperPacketsSent.Init
	}

	return nil
}
