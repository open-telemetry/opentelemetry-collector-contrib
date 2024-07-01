package daemonset

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for daemonsets
const (
	// Errors
	daemonSetPayloadErrorMessage = "No metrics related to DaemonSets found in Payload"
	// Metrics
	daemonSetMetricCurrentScheduled = "ddk8s.daemonset.current_scheduled_nodes"
	daemonSetMetricDesiredScheduled = "ddk8s.daemonset.desired_scheduled_nodes"
	daemonSetMetricMisscheduled     = "ddk8s.daemonset.misscheduled_nodes"
	daemonSetMetricReady            = "ddk8s.daemonset.ready_nodes"
	daemonSetMetricAvailable        = "ddk8s.daemonset.available_nodes"
	daemonSetMetricUnavailable      = "ddk8s.daemonset.unavailable_nodes"
	daemonSetMetricUpdatedScheduled = "ddk8s.daemonset.updated_scheduled_nodes"
	// Attributes
	daemonSetMetricUID         = "ddk8s.daemonset.uid"
	daemonSetMetricName        = "ddk8s.daemonset.name"
	daemonSetMetricLabels      = "ddk8s.daemonset.labels"
	daemonSetMetricAnnotations = "ddk8s.daemonset.annotations"
	daemonSetMetricFinalizers  = "ddk8s.daemonset.finalizers"
	daemonSetMetricCreateTime  = "ddk8s.daemonset.create_time"
	namespaceMetricName        = "ddk8s.namespace.name"
	namespaceMetricClusterID   = "ddk8s.cluster.id"
	namespaceMetricClusterName = "ddk8s.cluster.name"
)

var daemonsetMetricsToExtract = []string{
	daemonSetMetricCurrentScheduled,
	daemonSetMetricDesiredScheduled,
	daemonSetMetricMisscheduled,
	daemonSetMetricReady,
	daemonSetMetricAvailable,
	daemonSetMetricUnavailable,
	daemonSetMetricUpdatedScheduled,
}

// GetOtlpExportReqFromDatadogDaemonSetData converts Datadog daemonset data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogDaemonSetData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorDaemonSet)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(daemonSetPayloadErrorMessage)
	}

	daemonsets := ddReq.GetDaemonSets()

	if len(daemonsets) == 0 {
		log.Println("no daemonsets found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(daemonSetPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, metricName := range daemonsetMetricsToExtract {
		for _, daemonset := range daemonsets {
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
			appendDaemonSetMetrics(&scopeMetrics, resourceAttributes, metricAttributes, daemonset, metricName, timestamp)
		}
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendDaemonSetMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, daemonset *processv1.DaemonSet, metricName string, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(metricName)

	var metricVal int64

	if metadata := daemonset.GetMetadata(); metadata != nil {
		resourceAttributes.PutStr(daemonSetMetricUID, metadata.GetUid())
		metricAttributes.PutStr(namespaceMetricName, metadata.GetNamespace())
		metricAttributes.PutStr(daemonSetMetricName, metadata.GetName())
		metricAttributes.PutStr(daemonSetMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(daemonSetMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(daemonSetMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))
		metricAttributes.PutInt(daemonSetMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))
	}

	status := daemonset.GetStatus()
	spec := daemonset.GetSpec()

	if status != nil {
		switch metricName {
		case daemonSetMetricCurrentScheduled:
			metricVal = int64(status.GetCurrentNumberScheduled())
		case daemonSetMetricDesiredScheduled:
			metricVal = int64(status.GetDesiredNumberScheduled())
		case daemonSetMetricMisscheduled:
			metricVal = int64(status.GetNumberMisscheduled())
		case daemonSetMetricReady:
			metricVal = int64(status.GetNumberReady())
		case daemonSetMetricAvailable:
			metricVal = int64(status.GetNumberAvailable())
		case daemonSetMetricUnavailable:
			metricVal = int64(status.GetNumberUnavailable())
		case daemonSetMetricUpdatedScheduled:
			metricVal = int64(status.GetUpdatedNumberScheduled())
		}
	}

	if spec != nil {
		metricAttributes.PutStr("ddk8s.daemonset.deployment_strategy", spec.GetDeploymentStrategy())
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

func setHostK8sAttributes(metricAttributes pcommon.Map, clusterName string, clusterID string) {
	metricAttributes.PutStr(namespaceMetricClusterID, clusterID)
	metricAttributes.PutStr(namespaceMetricClusterName, clusterName)
}
