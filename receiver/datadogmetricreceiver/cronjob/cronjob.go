package cronjob

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Private constants for cron jobs
const (
	// Errors
	cronJobPayloadErrorMessage = "No metrics related to CronJobs found in Payload"
	// Metrics
	cronJobMetricActiveJobs = "ddk8s.cronjob.active.jobs"
	// Attributes
	cronJobMetricUID              = "ddk8s.cronjob.uid"
	cronJobMetricName             = "ddk8s.cronjob.name"
	cronJobMetricLabels           = "ddk8s.cronjob.labels"
	cronJobMetricAnnotations      = "ddk8s.cronjob.annotations"
	cronJobMetricFinalizers       = "ddk8s.cronjob.finalizers"
	cronJobMetricCreateTime       = "ddk8s.cronjob.create_time"
	namespaceMetricName           = "ddk8s.namespace.name"
	namespaceMetricClusterID      = "ddk8s.cluster.id"
	namespaceMetricClusterName    = "ddk8s.cluster.name"
	cronJobMetricSchedule         = "ddk8s.cronjob.schedule"
	cronJobMetricLastScheduleTime = "ddk8s.cronjob.last_schedule_time"
)

var cronjobMetricsToExtract = []string{
	cronJobMetricActiveJobs,
}

// GetOtlpExportReqFromDatadogCronJobData converts Datadog cron job data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogCronJobData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorCronJob)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(cronJobPayloadErrorMessage)
	}
	cronjobs := ddReq.GetCronJobs()

	if len(cronjobs) == 0 {
		log.Println("no cronjobs found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(cronJobPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, metricName := range cronjobMetricsToExtract {
		for _, cronjob := range cronjobs {
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
			appendCronJobMetrics(&scopeMetrics, resourceAttributes, metricAttributes, metricName, cronjob, timestamp)
		}
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendCronJobMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, metricName string, cronjob *processv1.CronJob, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(metricName)

	var metricVal int64

	metadata := cronjob.GetMetadata()
	resourceAttributes.PutStr(cronJobMetricUID, metadata.GetUid())
	metricAttributes.PutStr(namespaceMetricName, metadata.GetNamespace())
	metricAttributes.PutStr(cronJobMetricName, metadata.GetName())
	metricAttributes.PutStr(cronJobMetricLabels, strings.Join(metadata.GetLabels(), "&"))
	metricAttributes.PutStr(cronJobMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
	metricAttributes.PutStr(cronJobMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))

	status := cronjob.GetStatus()
	spec := cronjob.GetSpec()

	switch metricName {
	case cronJobMetricActiveJobs:
		metricVal = int64(len(status.GetActive()))
	}
	metricAttributes.PutStr(cronJobMetricSchedule, spec.GetSchedule())
	metricAttributes.PutInt(cronJobMetricLastScheduleTime, int64(status.GetLastScheduleTime())*1000)
	metricAttributes.PutInt(cronJobMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))

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