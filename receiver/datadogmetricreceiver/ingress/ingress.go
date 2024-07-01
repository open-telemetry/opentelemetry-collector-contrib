package ingress

import (
	"fmt"

	processv1 "github.com/DataDog/agent-payload/v5/process"
	//"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver"
	"log"
	"strings"
	//"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
)

const (
	// Attribute keys
	attrUID                    = "ddk8s.ingress.uid"
	attrNamespace              = "ddk8s.ingress.namespace"
	attrClusterID              = "ddk8s.ingress.cluster.id"
	attrClusterName            = "ddk8s.ingress.cluster.name"
	attrName                   = "ddk8s.ingress.name"
	attrLabels                 = "ddk8s.ingress.labels"
	attrAnnotations            = "ddk8s.ingress.annotations"
	attrFinalizers             = "ddk8s.ingress.finalizers"
	attrRules                  = "ddk8s.ingress.rules"
	attrType                   = "ddk8s.ingress.type"
	attrCreateTime             = "ddk8s.ingress.create_time"
	IngressPayloadErrorMessage = "No metrics related to Ingress found in Payload"
	// Metric names
	IngressmetricRuleCount = "ddk8s.ingress.rule_count"
)

func GetOtlpExportReqFromDatadogIngressData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorIngress)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(IngressPayloadErrorMessage)
	}
	ingresses := ddReq.GetIngresses()

	if len(ingresses) == 0 {
		log.Println("no ingresses found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(IngressPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	cluster_name := ddReq.GetClusterName()
	cluster_id := ddReq.GetClusterId()

	for _, ingress := range ingresses {
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
		setHostK8sAttributes(metricAttributes, cluster_name, cluster_id)
		appendMetrics(&scopeMetrics, resourceAttributes, metricAttributes, ingress, timestamp)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, ingress *processv1.Ingress, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(IngressmetricRuleCount)

	var metricVal int64

	if metadata := ingress.GetMetadata(); metadata != nil {
		resourceAttributes.PutStr(attrUID, metadata.GetUid())
		metricAttributes.PutStr(attrNamespace, metadata.GetNamespace())
		metricAttributes.PutStr(attrName, metadata.GetName())
		metricAttributes.PutStr(attrLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(attrAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(attrFinalizers, strings.Join(metadata.GetFinalizers(), ","))
		metricAttributes.PutInt(attrCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))
		metricAttributes.PutStr(attrType, "Ingress")

		if specDetails := ingress.GetSpec(); specDetails != nil {
			if rules := specDetails.GetRules(); rules != nil {
				metricVal = int64(len(rules))
				metricAttributes.PutStr(attrRules, convertIngressRulesToString(rules))
			}
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

func convertIngressRulesToString(rules []*processv1.IngressRule) string {
	var result strings.Builder

	for i, rule := range rules {
		if i > 0 {
			result.WriteString(";")
		}

		result.WriteString("host=")
		result.WriteString(rule.GetHost())

		result.WriteString("&http=(paths=")
		for j, path := range rule.GetHttpPaths() {
			if j > 0 {
				result.WriteString("&")
			}

			result.WriteString("(path=")
			result.WriteString(path.GetPath())
			result.WriteString("&pathType=")
			result.WriteString(path.GetPathType())
			result.WriteString("&backend=(service=(name=")
			result.WriteString(path.GetBackend().GetService().GetServiceName())
			result.WriteString("&port=(number=")
			result.WriteString(fmt.Sprintf("%d", path.GetBackend().GetService().GetPortNumber()))
			result.WriteString(")))")
		}
		result.WriteString(")")
	}

	return result.String()
}

func setHostK8sAttributes(metricAttributes pcommon.Map, cluster_name string, cluster_id string) {
	metricAttributes.PutStr(attrClusterID, cluster_id)
	metricAttributes.PutStr(attrClusterName, cluster_name)
}

