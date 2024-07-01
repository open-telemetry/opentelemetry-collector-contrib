package statefulset

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for statefulsets
const (
	// Errors
	statefulSetPayloadErrorMessage = "No metrics related to StatefulSets found in Payload"
	// Metrics
	statefulSetMetricAvailable = "ddk8s.statefulset.available"
	statefulSetMetricDesired   = "ddk8s.statefulset.desired"
	statefulSetMetricReady     = "ddk8s.statefulset.ready"
	statefulSetMetricUpdated   = "ddk8s.statefulset.updated"
	// Attributes
	statefulSetMetricUID         = "ddk8s.statefulset.uid"
	statefulSetMetricName        = "ddk8s.statefulset.name"
	statefulSetMetricLabels      = "ddk8s.statefulset.labels"
	statefulSetMetricAnnotations = "ddk8s.statefulset.annotations"
	statefulSetMetricFinalizers  = "ddk8s.statefulset.finalizers"
	statefulSetMetricCreateTime  = "ddk8s.statefulset.create_time"
	namespaceMetricName          = "ddk8s.namespace.name"
	namespaceMetricClusterID     = "ddk8s.cluster.id"
	namespaceMetricClusterName   = "ddk8s.cluster.name"
)

var statefulSetMetricsToExtract = []string{
	statefulSetMetricAvailable,
	statefulSetMetricDesired,
	statefulSetMetricReady,
	statefulSetMetricUpdated,
}

// GetOtlpExportReqFromDatadogStatefulSetData converts Datadog statefulset data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogStatefulSetData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorStatefulSet)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(statefulSetPayloadErrorMessage)
	}
	statefulsets := ddReq.GetStatefulSets()

	if len(statefulsets) == 0 {
		log.Println("no statefulsets found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(statefulSetPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, metricName := range statefulSetMetricsToExtract {
		for _, statefulset := range statefulsets {
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
			appendStatefulSetMetrics(&scopeMetrics, resourceAttributes, metricAttributes, statefulset, metricName, timestamp)
		}
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendStatefulSetMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, statefulset *processv1.StatefulSet, metricName string, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(metricName)

	var metricVal int64

	if metadata := statefulset.GetMetadata(); metadata != nil {
		resourceAttributes.PutStr(statefulSetMetricUID, metadata.GetUid())
		metricAttributes.PutStr(namespaceMetricName, metadata.GetNamespace())
		metricAttributes.PutStr(statefulSetMetricName, metadata.GetName())
		metricAttributes.PutStr(statefulSetMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(statefulSetMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(statefulSetMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))
		metricAttributes.PutInt(statefulSetMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))
	}

	status := statefulset.GetStatus()
	spec := statefulset.GetSpec()

	switch metricName {
	case statefulSetMetricAvailable:
		metricVal = int64(status.GetCurrentReplicas())
	case statefulSetMetricReady:
		metricVal = int64(status.GetReadyReplicas())
	case statefulSetMetricUpdated:
		metricVal = int64(status.GetUpdatedReplicas())
	case statefulSetMetricDesired:
		metricVal = int64(spec.GetDesiredReplicas())
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
