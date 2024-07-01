package cluster

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for clusters
const (
	// Errors
	clusterPayloadErrorMessage = "No metrics related to Clusters found in Payload"
	// Metrics
	clusterMetricNodeCount = "ddk8s.cluster.node_count"
	// Attributes
	clusterMetricUID             = "ddk8s.cluster.uid"
	clusterAttrClusterID         = "ddk8s.cluster.id"
	clusterAttrClusterName       = "ddk8s.cluster.name"
	clusterAttrKubeClusterName   = "kube_cluster_name"
	clusterAttrResourceVersion   = "ddk8s.cluster.resource_version"
	clusterAttrCPUCapacity       = "ddk8s.cluster.cpu_capacity"
	clusterAttrCPUAllocatable    = "ddk8s.cluster.cpu_allocatable"
	clusterAttrMemoryCapacity    = "ddk8s.cluster.memory_capacity"
	clusterAttrMemoryAllocatable = "ddk8s.cluster.memory_allocatable"
	clusterAttrTags              = "ddk8s.cluster.tags"
	clusterMetricCreateTime      = "ddk8s.cluster.create_time"
)

// GetOtlpExportReqFromClusterData converts Datadog cluster data into OTLP ExportRequest.
func GetOtlpExportReqFromClusterData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorCluster)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(clusterPayloadErrorMessage)
	}
	cluster := ddReq.GetCluster()

	if cluster == nil {
		log.Println("no clusters data found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(clusterPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	rm := resourceMetrics.AppendEmpty()
	resourceAttributes := rm.Resource().Attributes()
	metricAttributes := pcommon.NewMap()
	commonResourceAttributes := helpers.CommonResourceAttributes{
		Origin:   origin,
		ApiKey:   key,
		MwSource: "datadog",
	}
	helpers.SetMetricResourceAttributes(resourceAttributes, commonResourceAttributes)

	scopeMetrics := helpers.AppendInstrScope(&rm)
	setHostK8sAttributes(metricAttributes, resourceAttributes, clusterName, clusterID)
	appendClusterMetrics(&scopeMetrics, resourceAttributes, metricAttributes, cluster, timestamp)

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendClusterMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, cluster *processv1.Cluster, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(clusterMetricNodeCount)

	metricAttributes.PutStr(clusterAttrResourceVersion, cluster.GetResourceVersion())
	metricAttributes.PutInt(clusterAttrCPUCapacity, int64(cluster.GetCpuCapacity()))
	metricAttributes.PutInt(clusterAttrCPUAllocatable, int64(cluster.GetCpuAllocatable()))
	metricAttributes.PutInt(clusterAttrMemoryCapacity, int64(cluster.GetMemoryCapacity()))
	metricAttributes.PutInt(clusterAttrMemoryAllocatable, int64(cluster.GetMemoryAllocatable()))
	metricAttributes.PutStr(clusterAttrTags, strings.Join(cluster.GetTags(), "&"))
	metricAttributes.PutInt(clusterMetricCreateTime, helpers.CalculateCreateTime(cluster.GetCreationTimestamp()))

	var dataPoints pmetric.NumberDataPointSlice
	gauge := scopeMetric.SetEmptyGauge()
	dataPoints = gauge.DataPoints()

	dp := dataPoints.AppendEmpty()
	dp.SetTimestamp(pcommon.Timestamp(timestamp))

	dp.SetIntValue(int64(cluster.GetNodeCount()))
	attributeMap := dp.Attributes()
	metricAttributes.CopyTo(attributeMap)
}

func setHostK8sAttributes(metricAttributes pcommon.Map, resourceAttributes pcommon.Map, clusterName string, clusterID string) {
	resourceAttributes.PutStr(clusterMetricUID, clusterID)
	metricAttributes.PutStr(clusterAttrClusterID, clusterID)
	metricAttributes.PutStr(clusterAttrClusterName, clusterName)
}
