package clusterrolebinding

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for cluster role bindings
const (
	//Errors
	clusterRoleBindingsPayloadErrorMessage = "No metrics related to ClusterRoleBindings found in Payload"
	//Metrics
	clusterRoleBindingsMetricSubjectCount = "ddk8s.clusterrolebindings.subject.count"
	//Attributes
	clusterRoleBindingsMetricUID         = "ddk8s.clusterrolebindings.uid"
	clusterRoleBindingsMetricNamespace   = "ddk8s.clusterrolebindings.namespace"
	clusterRoleBindingsAttrClusterID     = "ddk8s.clusterrolebindings.cluster.id"
	clusterRoleBindingsAttrClusterName   = "ddk8s.clusterrolebindings.cluster.name"
	clusterRoleBindingsMetricName        = "ddk8s.clusterrolebindings.name"
	clusterRoleBindingsMetricCreateTime  = "ddk8s.clusterrolebindings.create_time"
	clusterRoleBindingsMetricSubjects    = "ddk8s.clusterrolebindings.subjects"
	clusterRoleBindingsMetricRoleRef     = "ddk8s.clusterrolebindings.roleref"
	clusterRoleBindingsMetricLabels      = "ddk8s.clusterrolebindings.labels"
	clusterRoleBindingsMetricAnnotations = "ddk8s.clusterrolebindings.annotations"
)

func GetOtlpExportReqFromDatadogClusterRoleBindingData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {

	ddReq, ok := Body.(*processv1.CollectorClusterRoleBinding)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(clusterRoleBindingsPayloadErrorMessage)
	}

	clusterRoleBindings := ddReq.GetClusterRoleBindings()
	if len(clusterRoleBindings) == 0 {
		log.Println("no cluster role bindings found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(clusterRoleBindingsPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, binding := range clusterRoleBindings {
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
		appendClusterRoleBindingMetrics(&scopeMetrics, resourceAttributes, metricAttributes, binding, timestamp)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendClusterRoleBindingMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, binding *processv1.ClusterRoleBinding, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(clusterRoleBindingsMetricSubjectCount)

	var metricVal int64

	if metadata := binding.GetMetadata(); metadata != nil {
		resourceAttributes.PutStr(clusterRoleBindingsMetricUID, metadata.GetUid())
		metricAttributes.PutStr(clusterRoleBindingsMetricNamespace, metadata.GetNamespace())
		metricAttributes.PutStr(clusterRoleBindingsMetricName, metadata.GetName())
		metricAttributes.PutStr(clusterRoleBindingsMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(clusterRoleBindingsMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(clusterRoleBindingsMetricRoleRef, getRoleRefString(binding.GetRoleRef()))
		metricAttributes.PutInt(clusterRoleBindingsMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))

		if subjects := binding.GetSubjects(); subjects != nil {
			metricAttributes.PutStr(clusterRoleBindingsMetricSubjects, convertSubjectsToString(subjects))
			metricVal = int64(len(subjects))
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
	metricAttributes.PutStr(clusterRoleBindingsAttrClusterID, clusterID)
	metricAttributes.PutStr(clusterRoleBindingsAttrClusterName, clusterName)
}

func convertSubjectsToString(subjects []*processv1.Subject) string {
	var result strings.Builder

	for i, subject := range subjects {
		if i > 0 {
			result.WriteString(";")
		}

		result.WriteString("kind=")
		result.WriteString(subject.GetKind())

		result.WriteString("&name=")
		result.WriteString(subject.GetName())

		result.WriteString("&namespace=")
		result.WriteString(subject.GetNamespace())
	}

	return result.String()
}

func getRoleRefString(ref *processv1.TypedLocalObjectReference) string {
	if ref == nil {
		return ""
	}
	return "apiGroup=" + ref.GetApiGroup() + "&kind=" + ref.GetKind() + "&name=" + ref.GetName()
}
