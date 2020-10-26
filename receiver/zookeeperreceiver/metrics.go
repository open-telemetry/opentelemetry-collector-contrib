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
	"go.opentelemetry.io/collector/consumer/pdata"
)

// Constants to define entries in the output of "mntr" command.

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

	serverStateResourceLabel = "server.state"

	maxMetricsLen = 6
)

func getOTLPMetricDescriptor(metric string) pdata.Metric {
	switch metric {
	case followersMetricKey:
		return followersDescriptor
	case syncedFollowersMetricKey:
		return syncedFollowersDescriptor
	case pendingSyncsMetricKey:
		return pendingSyncsDescriptor
	case avgLatencyMetricKey:
		return avgLatencyDescriptor
	case maxLatencyMetricKey:
		return maxLatencyDescriptor
	case minLatencyMetricKey:
		return minLatencyDescriptor
	case numAliveConnectionsMetricKey:
		return numAliveConnectionsDescriptor
	case outstandingRequestsMetricKey:
		return outstandingRequestsDescriptor
	case zNodeCountMetricKey:
		return zNodeCountDescriptor
	case watchCountMetricKey:
		return watchCountDescriptor
	case ephemeralsCountMetricKey:
		return ephemeralsCountDescriptor
	case approximateDataSizeMetricKey:
		return approximateDataSizeDescriptor
	case openFileDescriptorCountMetricKey:
		return openFileDescriptorCountDescriptor
	case maxFileDescriptorCountMetricKey:
		return maxFileDescriptorCountDescriptor
	}

	return pdata.NewMetric()
}

var followersDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.followers")
	metric.SetDescription("The number of followers. Only exposed by the leader.")
	metric.SetUnit("1")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var syncedFollowersDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.synced_followers")
	metric.SetDescription("The number of followers in sync with the leader. Only exposed by the leader.")
	metric.SetUnit("1")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var pendingSyncsDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.pending_syncs")
	metric.SetDescription("The number of pending syncs from the followers. Only exposed by the leader.")
	metric.SetUnit("1")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var avgLatencyDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.latency.avg")
	metric.SetDescription("Average time in milliseconds for requests to be processed.")
	metric.SetUnit("ms")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var maxLatencyDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.latency.max")
	metric.SetDescription("Maximum time in milliseconds for requests to be processed.")
	metric.SetUnit("ms")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var minLatencyDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.latency.min")
	metric.SetDescription("Minimum time in milliseconds for requests to be processed")
	metric.SetUnit("ms")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var numAliveConnectionsDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.connections_alive")
	metric.SetDescription("Number of active clients connected to a ZooKeeper server.")
	metric.SetUnit("1")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var outstandingRequestsDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.outstanding_requests")
	metric.SetDescription("Number of currently executing requests.")
	metric.SetUnit("1")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var zNodeCountDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.znodes")
	metric.SetDescription("Number of z-nodes that a ZooKeeper server has in its data tree.")
	metric.SetUnit("1")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var watchCountDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.watches")
	metric.SetDescription("Number of watches placed on Z-Nodes on a ZooKeeper server.")
	metric.SetUnit("1")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var ephemeralsCountDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.ephemeral_nodes")
	metric.SetDescription("Number of ephemeral nodes that a ZooKeeper server has in its data tree.")
	metric.SetUnit("1")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var approximateDataSizeDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.approximate_date_size")
	metric.SetDescription("Size of data in bytes that a ZooKeeper server has in its data tree.")
	metric.SetUnit("By")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var openFileDescriptorCountDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.open_file_descriptors")
	metric.SetDescription("Number of file descriptors that a ZooKeeper server has open.")
	metric.SetUnit("1")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()

var maxFileDescriptorCountDescriptor = func() pdata.Metric {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("zookeeper.max_file_descriptors")
	metric.SetDescription("Maximum number of file descriptors that a ZooKeeper server can open.")
	metric.SetUnit("1")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	return metric
}()
