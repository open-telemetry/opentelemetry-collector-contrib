package clusterroles

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for cluster roles
const (
	// Error
	clusterRolePayloadErrorMessage = "No metrics related to ClusterRoles found in Payload"
	// Metrics
	clusterRoleMetricRuleCount = "ddk8s.clusterrole.count"
	// Attributes
	clusterRoleMetricUID         = "ddk8s.clusterrole.uid"
	clusterRoleMetricNamespace   = "ddk8s.clusterrole.namespace"
	clusterRoleAttrClusterID     = "ddk8s.clusterrole.cluster.id"
	clusterRoleAttrClusterName   = "ddk8s.clusterrole.cluster.name"
	clusterRoleMetricName        = "ddk8s.clusterrole.name"
	clusterRoleMetricCreateTime  = "ddk8s.clusterrole.create.time"
	clusterRoleMetricLabels      = "ddk8s.clusterrole.labels"
	clusterRoleMetricAnnotations = "ddk8s.clusterrole.annotations"
	clusterRoleMetricType        = "ddk8s.clusterrole.type"
	clusterRoleMetricRules       = "ddk8s.clusterrole.rules"
)

func GetOtlpExportReqFromDatadogClusterRolesData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {

	ddReq, ok := Body.(*processv1.CollectorClusterRole)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(clusterRolePayloadErrorMessage)
	}

	croles := ddReq.GetClusterRoles()

	if len(croles) == 0 {
		log.Println("no croles found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(clusterRolePayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, role := range croles {
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
		appendClusterRoleMetrics(&scopeMetrics, resourceAttributes, metricAttributes, role, timestamp)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendClusterRoleMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, role *processv1.ClusterRole, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(clusterRoleMetricRuleCount)

	var metricVal int64

	if metadata := role.GetMetadata(); metadata != nil {
		resourceAttributes.PutStr(clusterRoleMetricUID, metadata.GetUid())
		metricAttributes.PutStr(clusterRoleMetricNamespace, metadata.GetNamespace())
		metricAttributes.PutStr(clusterRoleMetricName, metadata.GetName())
		metricAttributes.PutStr(clusterRoleMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(clusterRoleMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(clusterRoleMetricAnnotations, strings.Join(metadata.GetFinalizers(), ","))
		metricAttributes.PutInt(clusterRoleMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))
		metricAttributes.PutStr(clusterRoleMetricType, "ClusterRole")

		if rules := role.GetRules(); rules != nil {
			metricAttributes.PutStr(clusterRoleMetricRules, convertRulesToString(rules))
			metricVal = int64(len(rules))
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
	metricAttributes.PutStr(clusterRoleAttrClusterID, clusterID)
	metricAttributes.PutStr(clusterRoleAttrClusterName, clusterName)
}

func convertRulesToString(rules []*processv1.PolicyRule) string {
	var result strings.Builder

	for i, rule := range rules {
		if i > 0 {
			result.WriteString(";")
		}

		result.WriteString("verbs=")
		result.WriteString(strings.Join(rule.GetVerbs(), ","))

		result.WriteString("&apiGroups=")
		result.WriteString(strings.Join(rule.GetApiGroups(), ","))

		result.WriteString("&resources=")
		result.WriteString(strings.Join(rule.GetResources(), ","))

		result.WriteString("&resourceNames=")
		result.WriteString(strings.Join(rule.GetResourceNames(), ","))

		result.WriteString("&nonResourceURLs=")
		result.WriteString(strings.Join(rule.GetNonResourceURLs(), ","))

	}

	return result.String()
}
