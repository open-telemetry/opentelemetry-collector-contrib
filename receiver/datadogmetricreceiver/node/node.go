package node

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for nodes
const (
	// Errors
	nodePayloadErrorMessage = "No metrics related to Nodes found in Payload"
	// Metrics
	nodeMetricMetadata = "ddk8s.node.metadata"
	// Attributes
	nodeName                = "ddk8s.node.name"
	nodeMetricNamespace     = "ddk8s.namespace.name"
	nodeAttrClusterID       = "ddk8s.cluster.id"
	nodeAttrClusterName     = "ddk8s.cluster.name"
	nodeAttrKubeClusterName = "kube_cluster_name"
	nodeMetricRoles         = "ddk8s.node.roles"
	nodeMetricLabels        = "ddk8s.node.labels"
	nodeMetricAnnotations   = "ddk8s.node.annotations"
	nodeMetricFinalizers    = "ddk8s.node.finalizers"
	nodeMetricIP            = "ddk8s.node.ip"
	nodeMetricHostName      = "ddk8s.node.host.name"
	nodeMetricCreateTime    = "ddk8s.node.create_time"
)

// GetOtlpExportReqFromDatadogRoleBindingData converts Datadog role binding data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogNodeData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorNode)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(nodePayloadErrorMessage)
	}
	nodes := ddReq.GetNodes()

	if len(nodes) == 0 {
		log.Println("no nodes found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(nodePayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, node := range nodes {
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
		setHostK8sAttributes(metricAttributes, clusterName, clusterID)
		appendNodeMetrics(&scopeMetrics, resourceAttributes, metricAttributes, node, timestamp)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendNodeMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, node *processv1.Node, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(nodeMetricMetadata)

	var metricVal int64

	if metadata := node.GetMetadata(); metadata != nil {
		resourceAttributes.PutStr(nodeName, metadata.GetName())
		metricAttributes.PutStr(nodeMetricNamespace, metadata.GetNamespace())
		metricAttributes.PutStr(nodeMetricRoles, strings.Join(node.GetRoles(), "&"))
		metricAttributes.PutStr(nodeMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(nodeMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(nodeMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))
		metricAttributes.PutStr(nodeMetricIP, getNodeInternalIP(node.GetStatus()))
		metricAttributes.PutStr(nodeMetricHostName, getNodeHostName(node.GetStatus()))
		metricAttributes.PutInt(nodeMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))
	}

	var dataPoints pmetric.NumberDataPointSlice
	gauge := scopeMetric.SetEmptyGauge()
	dataPoints = gauge.DataPoints()
	dp := dataPoints.AppendEmpty()

	dp.SetTimestamp(pcommon.Timestamp(timestamp))
	dp.SetIntValue(metricVal)

	attributeMap := dp.Attributes()
	metricAttributes.CopyTo(attributeMap)
}

func getNodeInternalIP(status *processv1.NodeStatus) string {
	if status == nil {
		return ""
	}
	addresses := status.GetNodeAddresses()
	if addresses == nil {
		return ""
	}
	return addresses["InternalIP"]
}

func getNodeHostName(status *processv1.NodeStatus) string {
	if status == nil {
		return ""
	}
	addresses := status.GetNodeAddresses()
	if addresses == nil {
		return ""
	}
	return addresses["Hostname"]
}

func setHostK8sAttributes(metricAttributes pcommon.Map, clusterName string, clusterID string) {
	metricAttributes.PutStr(nodeAttrClusterID, clusterID)
	metricAttributes.PutStr(nodeAttrClusterName, clusterName)
}
