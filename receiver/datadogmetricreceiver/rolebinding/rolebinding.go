package rolebinding

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for role bindings
const (
	// Errors
	roleBindingsPayloadErrorMessage = "No metrics related to RoleBindings found in Payload"
	// Metrics
	roleBindingsMetricSubjectCount = "ddk8s.rolebindings.subject.count"
	// Attributes
	roleBindingsMetricUID         = "ddk8s.rolebindings.uid"
	roleBindingsMetricNamespace   = "ddk8s.rolebindings.namespace"
	roleBindingsAttrClusterID     = "ddk8s.rolebindings.cluster.id"
	roleBindingsAttrClusterName   = "ddk8s.rolebindings.cluster.name"
	roleBindingsMetricName        = "ddk8s.rolebindings.name"
	roleBindingsMetricCreateTime  = "ddk8s.rolebindings.create_time"
	roleBindingsMetricSubjects    = "ddk8s.rolebindings.subjects"
	roleBindingsMetricRoleRef     = "ddk8s.rolebindings.roleref"
	roleBindingsMetricType        = "ddk8s.rolebindings.type"
	roleBindingsMetricLabels      = "ddk8s.rolebindings.labels"
	roleBindingsMetricAnnotations = "ddk8s.rolebindings.annotations"
)

// GetOtlpExportReqFromDatadogRoleBindingData converts Datadog role binding data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogRoleBindingData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {

	ddReq, ok := Body.(*processv1.CollectorRoleBinding)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(roleBindingsPayloadErrorMessage)
	}
	
	roleBindings := ddReq.GetRoleBindings()

	if len(roleBindings) == 0 {
		log.Println("no role bindings found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(roleBindingsPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, binding := range roleBindings {
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
		appendRoleBindingMetrics(&scopeMetrics, resourceAttributes, metricAttributes, binding, timestamp)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendRoleBindingMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, binding *processv1.RoleBinding, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(roleBindingsMetricSubjectCount)

	var metricVal int64

	if metadata := binding.GetMetadata(); metadata != nil {
		resourceAttributes.PutStr(roleBindingsMetricUID, metadata.GetUid())
		metricAttributes.PutStr(roleBindingsMetricNamespace, metadata.GetNamespace())
		metricAttributes.PutStr(roleBindingsMetricName, metadata.GetName())
		metricAttributes.PutStr(roleBindingsMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(roleBindingsMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(roleBindingsMetricRoleRef, getRoleRefString(binding.GetRoleRef()))
		metricAttributes.PutInt(roleBindingsMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))

		if subjects := binding.GetSubjects(); subjects != nil {
			metricAttributes.PutStr(roleBindingsMetricSubjects, convertSubjectsToString(subjects))
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
	metricAttributes.PutStr(roleBindingsAttrClusterID, clusterID)
	metricAttributes.PutStr(roleBindingsAttrClusterName, clusterName)
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
