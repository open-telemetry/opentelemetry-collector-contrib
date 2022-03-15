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
	"fmt"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

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

// metricCreator handles generation of metric and metric recording
type metricCreator struct {
	computedMetricStore map[string]int64
	rb                  *metadata.ResourceBuilder
}

func newMetricCreator(rb *metadata.ResourceBuilder) *metricCreator {
	return &metricCreator{
		computedMetricStore: make(map[string]int64),
		rb:                  rb,
	}
}

func (m *metricCreator) recordDataPointsFunc(metric string) func(ts pdata.Timestamp, val int64) {
	switch metric {
	case followersMetricKey:
		return func(ts pdata.Timestamp, val int64) {
			m.computedMetricStore[followersMetricKey] = val
		}
	case syncedFollowersMetricKey:
		return func(ts pdata.Timestamp, val int64) {
			m.computedMetricStore[syncedFollowersMetricKey] = val
			m.rb.RecordZookeeperFollowerCountDataPoint(ts, val, metadata.AttributeState.Synced)
		}
	case pendingSyncsMetricKey:
		return m.rb.RecordZookeeperSyncPendingDataPoint
	case avgLatencyMetricKey:
		return m.rb.RecordZookeeperLatencyAvgDataPoint
	case maxLatencyMetricKey:
		return m.rb.RecordZookeeperLatencyMaxDataPoint
	case minLatencyMetricKey:
		return m.rb.RecordZookeeperLatencyMinDataPoint
	case numAliveConnectionsMetricKey:
		return m.rb.RecordZookeeperConnectionActiveDataPoint
	case outstandingRequestsMetricKey:
		return m.rb.RecordZookeeperRequestActiveDataPoint
	case zNodeCountMetricKey:
		return m.rb.RecordZookeeperZnodeCountDataPoint
	case watchCountMetricKey:
		return m.rb.RecordZookeeperWatchCountDataPoint
	case ephemeralsCountMetricKey:
		return m.rb.RecordZookeeperDataTreeEphemeralNodeCountDataPoint
	case approximateDataSizeMetricKey:
		return m.rb.RecordZookeeperDataTreeSizeDataPoint
	case openFileDescriptorCountMetricKey:
		return m.rb.RecordZookeeperFileDescriptorOpenDataPoint
	case maxFileDescriptorCountMetricKey:
		return m.rb.RecordZookeeperFileDescriptorLimitDataPoint
	case fSyncThresholdExceedCountMetricKey:
		return m.rb.RecordZookeeperFsyncExceededThresholdCountDataPoint
	case packetsReceivedMetricKey:
		return func(ts pdata.Timestamp, val int64) {
			m.rb.RecordZookeeperPacketCountDataPoint(ts, val, metadata.AttributeDirection.Received)
		}
	case packetsSentMetricKey:
		return func(ts pdata.Timestamp, val int64) {
			m.rb.RecordZookeeperPacketCountDataPoint(ts, val, metadata.AttributeDirection.Sent)
		}
	}

	return nil
}

func (m *metricCreator) generateComputedMetrics(logger *zap.Logger, ts pdata.Timestamp) {
	// not_synced Followers Count
	if err := m.computeNotSyncedFollowersMetric(ts); err != nil {
		logger.Debug("metric computation failed", zap.Error(err))
	}

}

func (m *metricCreator) computeNotSyncedFollowersMetric(ts pdata.Timestamp) error {
	followersTotal, ok := m.computedMetricStore[followersMetricKey]
	if !ok {
		return fmt.Errorf("could not compute not_synced follower.count, missing %s", followersMetricKey)
	}

	syncedFollowers, ok := m.computedMetricStore[syncedFollowersMetricKey]
	if !ok {
		return fmt.Errorf("could not compute not_synced follower.count, missing %s", syncedFollowersMetricKey)
	}

	val := followersTotal - syncedFollowers
	m.rb.RecordZookeeperFollowerCountDataPoint(ts, val, metadata.AttributeState.Unsynced)

	return nil
}
