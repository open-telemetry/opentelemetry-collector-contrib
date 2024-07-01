package replicaset

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for replica sets
const (
	// Errors
	replicaSetPayloadErrorMessage = "No metrics related to ReplicaSets found in Payload"
	// Metrics
	replicaSetMetricAvailable = "ddk8s.replicaset.available"
	replicaSetMetricDesired   = "ddk8s.replicaset.desired"
	replicaSetMetricReady     = "ddk8s.replicaset.ready"
	// Attributes
	replicaSetMetricUID         = "ddk8s.replicaset.uid"
	replicaSetMetricName        = "ddk8s.replicaset.name"
	replicaSetMetricLabels      = "ddk8s.replicaset.labels"
	replicaSetMetricAnnotations = "ddk8s.replicaset.annotations"
	replicaSetMetricFinalizers  = "ddk8s.replicaset.finalizers"
	replicaSetMetricCreateTime  = "ddk8s.replicaset.create_time"
	namespaceMetricName         = "ddk8s.namespace.name"
	namespaceMetricClusterID    = "ddk8s.cluster.id"
	namespaceMetricClusterName  = "ddk8s.cluster.name"
)

// GetOtlpExportReqFromDatadogReplicaSetData converts Datadog replica set data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogReplicaSetData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorReplicaSet)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(replicaSetPayloadErrorMessage)
	}
	replicasets := ddReq.GetReplicaSets()

	if len(replicasets) == 0 {
		log.Println("no replicasets found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(replicaSetPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, metricName := range []string{replicaSetMetricAvailable, replicaSetMetricDesired, replicaSetMetricReady} {
		for _, replicaset := range replicasets {
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
			appendReplicaSetMetrics(&scopeMetrics, resourceAttributes, metricAttributes, replicaset, metricName, timestamp)
		}
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendReplicaSetMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, replicaset *processv1.ReplicaSet, metricName string, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(metricName)

	var metricVal int64

	if metadata := replicaset.GetMetadata(); metadata != nil {
		resourceAttributes.PutStr(replicaSetMetricUID, metadata.GetUid())
		metricAttributes.PutStr(namespaceMetricName, metadata.GetNamespace())
		metricAttributes.PutStr(replicaSetMetricName, metadata.GetName())
		metricAttributes.PutStr(replicaSetMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(replicaSetMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(replicaSetMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))
		metricAttributes.PutInt(replicaSetMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))

		switch metricName {
		case replicaSetMetricAvailable:
			metricVal = int64(replicaset.GetAvailableReplicas())
		case replicaSetMetricDesired:
			metricVal = int64(replicaset.GetReplicasDesired())
		case replicaSetMetricReady:
			metricVal = int64(replicaset.GetReadyReplicas())
		}
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
