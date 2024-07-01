package job

import (
	"log"
	"strings"

	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
)

// Private constants for jobs
const (
	// Errors
	jobPayloadErrorMessage = "No metrics related to Jobs found in Payload"
	// Metrics
	jobMetricActivePods            = "ddk8s.job.active_pods"
	jobMetricDesiredSuccessfulPods = "ddk8s.job.desired_successful_pods"
	jobMetricFailedPods            = "ddk8s.job.failed_pods"
	jobMetricMaxParallelPods       = "ddk8s.job.max_parallel_pods"
	jobMetricSuccessfulPods        = "ddk8s.job.successful_pods"
	// Attributes
	jobMetricUID               = "ddk8s.job.uid"
	jobMetricName              = "ddk8s.job.name"
	jobMetricLabels            = "ddk8s.job.labels"
	jobMetricAnnotations       = "ddk8s.job.annotations"
	jobMetricFinalizers        = "ddk8s.job.finalizers"
	jobMetricCreateTime        = "ddk8s.job.create_time"
	namespaceMetricName        = "ddk8s.namespace.name"
	namespaceMetricClusterID   = "ddk8s.cluster.id"
	namespaceMetricClusterName = "ddk8s.cluster.name"
)

var jobMetricsToExtract = []string{
	jobMetricActivePods,
	jobMetricDesiredSuccessfulPods,
	jobMetricFailedPods,
	jobMetricMaxParallelPods,
	jobMetricSuccessfulPods,
}

// GetOtlpExportReqFromDatadogJobData converts Datadog job data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogJobData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorJob)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(jobPayloadErrorMessage)
	}

	jobs := ddReq.GetJobs()

	if len(jobs) == 0 {
		log.Println("no jobs found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(jobPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()
	for _, metricName := range jobMetricsToExtract {
		for _, job := range jobs {
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
			appendJobMetrics(&scopeMetrics, resourceAttributes, metricAttributes, metricName, job, timestamp)
		}
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendJobMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, metricName string, job *processv1.Job, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(metricName)

	var metricVal int64

	metadata := job.GetMetadata()
	resourceAttributes.PutStr(jobMetricUID, metadata.GetUid())
	metricAttributes.PutStr(namespaceMetricName, metadata.GetNamespace())
	metricAttributes.PutStr(jobMetricName, metadata.GetName())
	metricAttributes.PutStr(jobMetricLabels, strings.Join(metadata.GetLabels(), "&"))
	metricAttributes.PutStr(jobMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
	metricAttributes.PutStr(jobMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))
	metricAttributes.PutInt(jobMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))

	status := job.GetStatus()
	spec := job.GetSpec()

	switch metricName {
	case jobMetricActivePods:
		metricVal = int64(status.GetActive())
	case jobMetricDesiredSuccessfulPods:
		metricVal = int64(spec.GetCompletions())
	case jobMetricFailedPods:
		metricVal = int64(status.GetFailed())
	case jobMetricMaxParallelPods:
		metricVal = int64(spec.GetParallelism())
	case jobMetricSuccessfulPods:
		metricVal = int64(status.GetSucceeded())
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
