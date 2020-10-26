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
// TODO: Add descriptors for all metrics (Will do it in a separate PR in an effort to keep the initial PR smaller/easier to review).
const (
	followersMetricKey       = "zk_followers"
	syncedFollowersMetricKey = "zk_synced_followers"
	pendingSyncsMetricKey    = "zk_pending_syncs"
	avgLatencyMetricKey      = "zk_avg_latency"
	maxLatencyMetricKey      = "zk_max_latency"
	minLatencyMetricKey      = "zk_min_latency"

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
