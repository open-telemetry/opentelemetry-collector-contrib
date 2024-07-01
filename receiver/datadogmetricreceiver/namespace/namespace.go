package namespace

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for namespaces
const (
	// Errors
	namespacePayloadErrorMessage = "No metrics related to Namespaces found in Payload"
	// Metrics
	namespaceMetricMetadata = "ddk8s.namespace.metadata"
	// Attributes
	namespaceMetricUID         = "ddk8s.namespace.uid"
	namespaceMetricName        = "ddk8s.namespace.name"
	namespaceMetricStatus      = "ddk8s.namespace.status"
	namespaceMetricLabels      = "ddk8s.namespace.labels"
	namespaceMetricAnnotations = "ddk8s.namespace.annotations"
	namespaceMetricFinalizers  = "ddk8s.namespace.finalizers"
	namespaceMetricCreateTime  = "ddk8s.namespace.create_time"
	namespaceAttrClusterID     = "ddk8s.cluster.id"
	namespaceAttrClusterName   = "ddk8s.cluster.name"
)

// GetOtlpExportReqFromNamespaceData converts Datadog namespace data into OTLP ExportRequest.
func GetOtlpExportReqFromNamespaceData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorNamespace)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(namespacePayloadErrorMessage)
	}

	namespaces := ddReq.GetNamespaces()

	if len(namespaces) == 0 {
		log.Println("no namespaces found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(namespacePayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, namespace := range namespaces {
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
		appendNamespaceMetrics(&scopeMetrics, resourceAttributes, metricAttributes, namespace, timestamp)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendNamespaceMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, namespace *processv1.Namespace, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(namespaceMetricMetadata)

	var metricVal int64

	if metadata := namespace.GetMetadata(); metadata != nil {
		resourceAttributes.PutStr(namespaceMetricUID, metadata.GetUid())
		metricAttributes.PutStr(namespaceMetricName, metadata.GetNamespace())
		metricAttributes.PutStr(namespaceMetricName, metadata.GetName())
		metricAttributes.PutStr(namespaceMetricStatus, namespace.GetStatus())
		metricAttributes.PutStr(namespaceMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(namespaceMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(namespaceMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))
		metricAttributes.PutInt(namespaceMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))
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
	metricAttributes.PutStr(namespaceAttrClusterID, clusterID)
	metricAttributes.PutStr(namespaceAttrClusterName, clusterName)
}
