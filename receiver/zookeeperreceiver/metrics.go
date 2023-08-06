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

	ruokKey = "ruok"
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

func (m *metricCreator) recordDataPointsFunc(rmb *metadata.ResourceMetricsBuilder, metric string) func(ts pcommon.Timestamp, val int64) {
	switch metric {
	case followersMetricKey:
		return func(ts pcommon.Timestamp, val int64) {
			m.computedMetricStore[followersMetricKey] = val
		}
	case syncedFollowersMetricKey:
		return func(ts pcommon.Timestamp, val int64) {
			m.computedMetricStore[syncedFollowersMetricKey] = val
			rmb.RecordZookeeperFollowerCountDataPoint(ts, val, metadata.AttributeStateSynced)
		}
	case pendingSyncsMetricKey:
		return rmb.RecordZookeeperSyncPendingDataPoint
	case avgLatencyMetricKey:
		return rmb.RecordZookeeperLatencyAvgDataPoint
	case maxLatencyMetricKey:
		return rmb.RecordZookeeperLatencyMaxDataPoint
	case minLatencyMetricKey:
		return rmb.RecordZookeeperLatencyMinDataPoint
	case numAliveConnectionsMetricKey:
		return rmb.RecordZookeeperConnectionActiveDataPoint
	case outstandingRequestsMetricKey:
		return rmb.RecordZookeeperRequestActiveDataPoint
	case zNodeCountMetricKey:
		return rmb.RecordZookeeperZnodeCountDataPoint
	case watchCountMetricKey:
		return rmb.RecordZookeeperWatchCountDataPoint
	case ephemeralsCountMetricKey:
		return rmb.RecordZookeeperDataTreeEphemeralNodeCountDataPoint
	case approximateDataSizeMetricKey:
		return rmb.RecordZookeeperDataTreeSizeDataPoint
	case openFileDescriptorCountMetricKey:
		return rmb.RecordZookeeperFileDescriptorOpenDataPoint
	case maxFileDescriptorCountMetricKey:
		return rmb.RecordZookeeperFileDescriptorLimitDataPoint
	case fSyncThresholdExceedCountMetricKey:
		return rmb.RecordZookeeperFsyncExceededThresholdCountDataPoint
	case ruokKey:
		return rmb.RecordZookeeperRuokDataPoint
	case packetsReceivedMetricKey:
		return func(ts pcommon.Timestamp, val int64) {
			rmb.RecordZookeeperPacketCountDataPoint(ts, val, metadata.AttributeDirectionReceived)
		}
	case packetsSentMetricKey:
		return func(ts pcommon.Timestamp, val int64) {
			rmb.RecordZookeeperPacketCountDataPoint(ts, val, metadata.AttributeDirectionSent)
		}
	}

	return nil
}

func (m *metricCreator) generateComputedMetrics(logger *zap.Logger, rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp) {
	// not_synced Followers Count
	if err := m.computeNotSyncedFollowersMetric(rmb, ts); err != nil {
		logger.Debug("metric computation failed", zap.Error(err))
	}

}

func (m *metricCreator) computeNotSyncedFollowersMetric(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp) error {
	followersTotal, ok := m.computedMetricStore[followersMetricKey]
	if !ok {
		return fmt.Errorf("could not compute not_synced follower.count, missing %s", followersMetricKey)
	}

	syncedFollowers, ok := m.computedMetricStore[syncedFollowersMetricKey]
	if !ok {
		return fmt.Errorf("could not compute not_synced follower.count, missing %s", syncedFollowersMetricKey)
	}

	val := followersTotal - syncedFollowers
	rmb.RecordZookeeperFollowerCountDataPoint(ts, val, metadata.AttributeStateUnsynced)

	return nil
}
