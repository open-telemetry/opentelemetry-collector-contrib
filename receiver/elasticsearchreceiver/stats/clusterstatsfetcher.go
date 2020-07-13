package stats

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/utils"
)

const (
	clusterStatusGreen  = "green"
	clusterStatusYellow = "yellow"
	clusterStatusRed    = "red"
)

// GetClusterStatsDatapoints fetches datapoints for ES cluster level stats
func GetClusterStatsDatapoints(clusterStatsOutput *ClusterStatsOutput, defaultDims map[string]string, enhanced bool) []*metricspb.Metric {
	var out []*metricspb.Metric

	if enhanced {
		out = append(out, []*metricspb.Metric{
			utils.PrepareGauge("elasticsearch.cluster.initializing-shards", defaultDims, clusterStatsOutput.InitializingShards),
			utils.PrepareGauge("elasticsearch.cluster.delayed-unassigned-shards", defaultDims, clusterStatsOutput.DelayedUnassignedShards),
			utils.PrepareGauge("elasticsearch.cluster.pending-tasks", defaultDims, clusterStatsOutput.NumberOfPendingTasks),
			utils.PrepareGauge("elasticsearch.cluster.in-flight-fetches", defaultDims, clusterStatsOutput.NumberOfInFlightFetch),
			utils.PrepareGauge("elasticsearch.cluster.task-max-wait-time", defaultDims, clusterStatsOutput.TaskMaxWaitingInQueueMillis),
			utils.PrepareGaugeF("elasticsearch.cluster.active-shards-percent", defaultDims, clusterStatsOutput.ActiveShardsPercentAsNumber),
			utils.PrepareGauge("elasticsearch.cluster.status", defaultDims, getMetricValueFromClusterStatus(clusterStatsOutput.Status)),
		}...)
	}
	out = append(out, []*metricspb.Metric{
		utils.PrepareGauge("elasticsearch.cluster.active-primary-shards", defaultDims, clusterStatsOutput.ActivePrimaryShards),
		utils.PrepareGauge("elasticsearch.cluster.active-shards", defaultDims, clusterStatsOutput.ActiveShards),
		utils.PrepareGauge("elasticsearch.cluster.number-of-data_nodes", defaultDims, clusterStatsOutput.NumberOfDataNodes),
		utils.PrepareGauge("elasticsearch.cluster.number-of-nodes", defaultDims, clusterStatsOutput.NumberOfNodes),
		utils.PrepareGauge("elasticsearch.cluster.relocating-shards", defaultDims, clusterStatsOutput.RelocatingShards),
		utils.PrepareGauge("elasticsearch.cluster.unassigned-shards", defaultDims, clusterStatsOutput.UnassignedShards),
	}...)
	return out
}

// Map cluster status to a numeric value
func getMetricValueFromClusterStatus(s *string) *int64 {
	// For whatever reason if the monitor did not get cluster status return nil
	if s == nil {
		return nil
	}
	out := new(int64)
	status := *s

	switch status {
	case clusterStatusGreen:
		*out = 0
	case clusterStatusYellow:
		*out = 1
	case clusterStatusRed:
		*out = 2
	}

	return out
}
