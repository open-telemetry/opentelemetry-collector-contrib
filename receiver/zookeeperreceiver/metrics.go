// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zookeeperreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
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
	mb                  *metadata.MetricsBuilder
	// Temporary feature gates while transitioning to metrics without a direction attribute
}

func newMetricCreator(mb *metadata.MetricsBuilder) *metricCreator {
	return &metricCreator{
		computedMetricStore: make(map[string]int64),
		mb:                  mb,
	}
}

func (m *metricCreator) recordDataPointsFunc(metric string) func(ts pcommon.Timestamp, val int64) {
	switch metric {
	case followersMetricKey:
		return func(ts pcommon.Timestamp, val int64) {
			m.computedMetricStore[followersMetricKey] = val
		}
	case syncedFollowersMetricKey:
		return func(ts pcommon.Timestamp, val int64) {
			m.computedMetricStore[syncedFollowersMetricKey] = val
			m.mb.RecordZookeeperFollowerCountDataPoint(ts, val, metadata.AttributeStateSynced)
		}
	case pendingSyncsMetricKey:
		return m.mb.RecordZookeeperSyncPendingDataPoint
	case avgLatencyMetricKey:
		return m.mb.RecordZookeeperLatencyAvgDataPoint
	case maxLatencyMetricKey:
		return m.mb.RecordZookeeperLatencyMaxDataPoint
	case minLatencyMetricKey:
		return m.mb.RecordZookeeperLatencyMinDataPoint
	case numAliveConnectionsMetricKey:
		return m.mb.RecordZookeeperConnectionActiveDataPoint
	case outstandingRequestsMetricKey:
		return m.mb.RecordZookeeperRequestActiveDataPoint
	case zNodeCountMetricKey:
		return m.mb.RecordZookeeperZnodeCountDataPoint
	case watchCountMetricKey:
		return m.mb.RecordZookeeperWatchCountDataPoint
	case ephemeralsCountMetricKey:
		return m.mb.RecordZookeeperDataTreeEphemeralNodeCountDataPoint
	case approximateDataSizeMetricKey:
		return m.mb.RecordZookeeperDataTreeSizeDataPoint
	case openFileDescriptorCountMetricKey:
		return m.mb.RecordZookeeperFileDescriptorOpenDataPoint
	case maxFileDescriptorCountMetricKey:
		return m.mb.RecordZookeeperFileDescriptorLimitDataPoint
	case fSyncThresholdExceedCountMetricKey:
		return m.mb.RecordZookeeperFsyncExceededThresholdCountDataPoint
	case packetsReceivedMetricKey:
		return func(ts pcommon.Timestamp, val int64) {
			m.mb.RecordZookeeperPacketCountDataPoint(ts, val, metadata.AttributeDirectionReceived)
		}
	case packetsSentMetricKey:
		return func(ts pcommon.Timestamp, val int64) {
			m.mb.RecordZookeeperPacketCountDataPoint(ts, val, metadata.AttributeDirectionSent)
		}
	}

	return nil
}

func (m *metricCreator) generateComputedMetrics(logger *zap.Logger, ts pcommon.Timestamp) {
	// not_synced Followers Count
	if err := m.computeNotSyncedFollowersMetric(ts); err != nil {
		logger.Debug("metric computation failed", zap.Error(err))
	}

}

func (m *metricCreator) computeNotSyncedFollowersMetric(ts pcommon.Timestamp) error {
	followersTotal, ok := m.computedMetricStore[followersMetricKey]
	if !ok {
		return fmt.Errorf("could not compute not_synced follower.count, missing %s", followersMetricKey)
	}

	syncedFollowers, ok := m.computedMetricStore[syncedFollowersMetricKey]
	if !ok {
		return fmt.Errorf("could not compute not_synced follower.count, missing %s", syncedFollowersMetricKey)
	}

	val := followersTotal - syncedFollowers
	m.mb.RecordZookeeperFollowerCountDataPoint(ts, val, metadata.AttributeStateUnsynced)

	return nil
}
