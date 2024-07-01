package hpa

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Constants for HPA metrics
const (
	// Metrics
	hpaMetricCurrentReplicas = "ddk8s.hpa.current_replicas"
	hpaMetricDesiredReplicas = "ddk8s.hpa.desired_replicas"
	hpaMetricMaxReplicas     = "ddk8s.hpa.max_replicas"
	hpaMetricMinReplicas     = "ddk8s.hpa.min_replicas"
	hpaMetricUID             = "ddk8s.hpa.uid"
	// Attributes
	hpaMetricName        = "ddk8s.hpa.name"
	hpaMetricNamespace   = "ddk8s.hpa.namespace"
	hpaMetricLabels      = "ddk8s.hpa.labels"
	hpaMetricAnnotations = "ddk8s.hpa.annotations"
	hpaMetricFinalizers  = "ddk8s.hpa.finalizers"
	hpaMetricClusterID   = "ddk8s.hpa.cluster.id"
	hpaMetricClusterName = "ddk8s.hpa.cluster.name"
	// Error
	ErrNoMetricsInPayload = "No metrics related to HPA found in Payload"
)

var hpaMetricsToExtract = []string{
	hpaMetricCurrentReplicas,
	hpaMetricDesiredReplicas,
	hpaMetricMaxReplicas,
	hpaMetricMinReplicas,
}

// GetOtlpExportReqFromDatadogHPAData converts Datadog HPA data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogHPAData(origin string, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {

	ddReq, ok := Body.(*processv1.CollectorHorizontalPodAutoscaler)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(ErrNoMetricsInPayload)
	}

	hpas := ddReq.GetHorizontalPodAutoscalers()

	if len(hpas) == 0 {
		log.Println("no hpas found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(ErrNoMetricsInPayload)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, metricName := range hpaMetricsToExtract {
		for _, hpa := range hpas {
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
			appendHPAMetrics(&scopeMetrics, resourceAttributes, metricAttributes, hpa, metricName, timestamp)
		}
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendHPAMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, hpa *processv1.HorizontalPodAutoscaler, metricName string, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(metricName)

	var metricVal int64

	metadata := hpa.GetMetadata()
	if metadata != nil {
		resourceAttributes.PutStr(hpaMetricUID, metadata.GetUid())
		metricAttributes.PutStr(hpaMetricNamespace, metadata.GetNamespace())
		metricAttributes.PutStr(hpaMetricName, metadata.GetName())
		metricAttributes.PutStr(hpaMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(hpaMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(hpaMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))
	}

	specDetails := hpa.GetSpec()
	statusDetails := hpa.GetStatus()

	switch metricName {
	case hpaMetricCurrentReplicas:
		metricVal = int64(statusDetails.GetCurrentReplicas())
	case hpaMetricDesiredReplicas:
		metricVal = int64(statusDetails.GetDesiredReplicas())
	case hpaMetricMaxReplicas:
		metricVal = int64(specDetails.GetMaxReplicas())
	case hpaMetricMinReplicas:
		metricVal = int64(specDetails.GetMinReplicas())
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
	metricAttributes.PutStr(hpaMetricClusterID, clusterID)
	metricAttributes.PutStr(hpaMetricClusterName, clusterName)
}
