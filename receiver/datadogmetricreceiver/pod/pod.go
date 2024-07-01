package pod

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for pods
const (
	// Errors
	podPayloadErrorMessage = "No metrics related to Pods found in Payload"
	// Metrics
	podMetricRestartCount = "ddk8s.pod.restart_count"
	// Attributes
	podName                = "ddk8s.pod.name"
	podMetricNamespace     = "ddk8s.namespace.name"
	podAttrClusterID       = "ddk8s.cluster.id"
	podAttrClusterName     = "ddk8s.cluster.name"
	podAttrKubeClusterName = "kube_cluster_name"
	podMetricIP            = "ddk8s.pod.ip"
	podMetricQOS           = "ddk8s.pod.qos"
	podMetricLabels        = "ddk8s.pod.labels"
	podMetricAnnotations   = "ddk8s.pod.annotations"
	podMetricFinalizers    = "ddk8s.pod.finalizers"
	podMetricCreateTime    = "ddk8s.pod.create_time"
)

// getOtlpExportReqFromPodData converts Datadog pod data into OTLP ExportRequest.
func GetOtlpExportReqFromPodData(origin string, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorPod)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(podPayloadErrorMessage)
	}
	pods := ddReq.GetPods()

	if len(pods) == 0 {
		log.Println("no pods found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(podPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, pod := range pods {
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
		appendPodMetrics(&scopeMetrics, resourceAttributes, metricAttributes, pod, timestamp)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendPodMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, pod *processv1.Pod, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(podMetricRestartCount)
	metadata := pod.GetMetadata()

	if metadata != nil {
		resourceAttributes.PutStr(podName, metadata.GetName())
		metricAttributes.PutStr(podMetricNamespace, metadata.GetNamespace())
		metricAttributes.PutStr(podMetricIP, pod.GetIP())
		metricAttributes.PutStr(podMetricQOS, pod.GetQOSClass())
		metricAttributes.PutStr(podMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(podMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(podMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))

		// Calculate pod creation time
		metricAttributes.PutInt(podMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))
	}

	var dataPoints pmetric.NumberDataPointSlice
	gauge := scopeMetric.SetEmptyGauge()
	dataPoints = gauge.DataPoints()
	dp := dataPoints.AppendEmpty()

	dp.SetTimestamp(pcommon.Timestamp(timestamp))
	dp.SetIntValue(int64(pod.GetRestartCount()))

	attributeMap := dp.Attributes()
	metricAttributes.CopyTo(attributeMap)
}

func setHostK8sAttributes(metricAttributes pcommon.Map, clusterName string, clusterID string) {
	metricAttributes.PutStr(podAttrClusterID, clusterID)
	metricAttributes.PutStr(podAttrClusterName, clusterName)
}
