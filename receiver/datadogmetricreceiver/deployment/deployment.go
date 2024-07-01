package deployment

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for deployments
const (
	// Errors
	deploymentPayloadErrorMessage = "No metrics related to Deployments found in Payload"
	// Metrics
	deploymentMetricDesired             = "ddk8s.deployment.desired"
	deploymentMetricAvailable           = "ddk8s.deployment.available"
	deploymentMetricReplicasUpdated     = "ddk8s.deployment.replicas.updated"
	deploymentMetricReplicasUnupdated   = "dk8s.deployment.replicas.unupdated"
	deploymentMetricReplicasAvailable   = "dk8s.deployment.replicas.available"
	deploymentMetricReplicasUnavailable = "dk8s.deployment.replicas.unavailable"
	// Attributes
	deploymentMetricUID          = "ddk8s.deployment.uid"
	deploymentMetricName         = "ddk8s.deployment.name"
	deploymentMetricLabels       = "ddk8s.deployment.labels"
	deploymentMetricAnnotations  = "ddk8s.deployment.annotations"
	deploymentDeploymentStrategy = "ddk8s.deployment.deployment_strategy"
	deploymentMetricFinalizers   = "ddk8s.deployment.finalizers"
	deploymentMetricCreateTime   = "ddk8s.deployment.create_time"
	namespaceMetricName          = "ddk8s.namespace.name"
	namespaceMetricClusterID     = "ddk8s.cluster.id"
	namespaceMetricClusterName   = "ddk8s.cluster.name"
)

var deploymentMetricsToExtract = []string{
	deploymentMetricDesired,
	deploymentMetricAvailable,
	deploymentMetricReplicasUpdated,
	deploymentMetricReplicasUnupdated,
	deploymentMetricReplicasAvailable,
	deploymentMetricReplicasUnavailable,
}

// GetOtlpExportReqFromDatadogDeploymentData converts Datadog deployment data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogDeploymentData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorDeployment)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(deploymentPayloadErrorMessage)
	}
	deployments := ddReq.GetDeployments()

	if len(deployments) == 0 {
		log.Println("no deployments found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(deploymentPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, metricName := range deploymentMetricsToExtract {
		for _, deployment := range deployments {
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
			appendDeploymentMetrics(&scopeMetrics, resourceAttributes, metricAttributes, deployment, metricName, timestamp)
		}
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendDeploymentMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, deployment *processv1.Deployment, metricName string, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(metricName)

	var metricVal int64

	if metadata := deployment.GetMetadata(); metadata != nil {
		resourceAttributes.PutStr(deploymentMetricUID, metadata.GetUid())
		metricAttributes.PutStr(namespaceMetricName, metadata.GetNamespace())
		metricAttributes.PutStr(deploymentMetricName, metadata.GetName())
		metricAttributes.PutStr(deploymentMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(deploymentMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(deploymentMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))
		metricAttributes.PutInt(deploymentMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))
		metricAttributes.PutStr(deploymentDeploymentStrategy, deployment.GetDeploymentStrategy())
	}

	switch metricName {
	case deploymentMetricDesired:
		metricVal = int64(deployment.GetReplicasDesired())
	case deploymentMetricAvailable:
		metricVal = int64(deployment.GetReplicas())
	case deploymentMetricReplicasUpdated:
		metricVal = int64(deployment.GetUpdatedReplicas())
	case deploymentMetricReplicasUnupdated:
		metricVal = int64(deployment.GetUpdatedReplicas()) // RECHECK THIS
	case deploymentMetricReplicasAvailable:
		metricVal = int64(deployment.GetAvailableReplicas())
	case deploymentMetricReplicasUnavailable:
		metricVal = int64(deployment.GetUnavailableReplicas())
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
