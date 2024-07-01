package serviceaccount

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for service accounts
const (
	// Errors
	serviceAccountPayloadErrorMessage = "No metrics related to ServiceAccounts found in Payload"
	// Metrics
	serviceAccountMetricSecretCount = "ddk8s.serviceaccount.secret.count"
	// Attributes
	serviceAccountMetricUID                          = "ddk8s.serviceaccount.uid"
	serviceAccountMetricNamespace                    = "ddk8s.serviceaccount.namespace"
	serviceAccountAttrClusterID                      = "ddk8s.serviceaccount.cluster.id"
	serviceAccountAttrClusterName                    = "ddk8s.serviceaccount.cluster.name"
	serviceAccountMetricName                         = "ddk8s.serviceaccount.name"
	serviceAccountMetricCreateTime                   = "ddk8s.serviceaccount.create_time"
	serviceAccountMetricSecrets                      = "ddk8s.serviceaccount.secrets"
	serviceAccountMetricLabels                       = "ddk8s.serviceaccount.labels"
	serviceAccountMetricAnnotations                  = "ddk8s.serviceaccount.annotations"
	serviceAccountMetricType                         = "ddk8s.serviceaccount.type"
	serviceAccountMetricAutomountServiceAccountToken = "ddk8s.serviceaccount.automount_serviceaccount_token"
)

// GetOtlpExportReqFromDatadogServiceAccountData converts Datadog service account data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogServiceAccountData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorServiceAccount)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(serviceAccountPayloadErrorMessage)
	}

	serviceAccounts := ddReq.GetServiceAccounts()

	if len(serviceAccounts) == 0 {
		log.Println("no service accounts found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(serviceAccountPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, account := range serviceAccounts {
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
		appendServiceAccountMetrics(&scopeMetrics, resourceAttributes, metricAttributes, account, timestamp)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendServiceAccountMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, account *processv1.ServiceAccount, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(serviceAccountMetricSecretCount)

	var metricVal int64

	if metadata := account.GetMetadata(); metadata != nil {
		resourceAttributes.PutStr(serviceAccountMetricUID, metadata.GetUid())
		metricAttributes.PutStr(serviceAccountMetricNamespace, metadata.GetNamespace())
		metricAttributes.PutStr(serviceAccountMetricName, metadata.GetName())
		metricAttributes.PutStr(serviceAccountMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(serviceAccountMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(serviceAccountMetricType, "ServiceAccount")
		metricAttributes.PutBool(serviceAccountMetricAutomountServiceAccountToken, account.GetAutomountServiceAccountToken())
		metricAttributes.PutInt(serviceAccountMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))

		if secrets := account.GetSecrets(); secrets != nil {
			metricAttributes.PutStr(serviceAccountMetricSecrets, convertSecretsToString(secrets))
			metricVal = int64(len(secrets))
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
	metricAttributes.PutStr(serviceAccountAttrClusterID, clusterID)
	metricAttributes.PutStr(serviceAccountAttrClusterName, clusterName)
}

func convertSecretsToString(secrets []*processv1.ObjectReference) string {
	var result strings.Builder

	for i, secret := range secrets {
		if i > 0 {
			result.WriteString(";")
		}

		result.WriteString("kind=")
		result.WriteString(secret.GetKind())

		result.WriteString("&name=")
		result.WriteString(secret.GetName())

		result.WriteString("&namespace=")
		result.WriteString(secret.GetNamespace())

	}

	return result.String()
}
