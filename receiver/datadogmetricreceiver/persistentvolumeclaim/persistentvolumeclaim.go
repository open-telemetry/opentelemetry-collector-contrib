package persistentvolumeclaim

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

const (
	// Errors
	pvcPayloadErrorMessage = "No metrics related to PersistentVolumeClaims found in Payload"
	// Metrics
	pvcMetricCapacity = "ddk8s.persistentvolumeclaim.capacity"
	// Attributes
	pvcMetricUID          = "ddk8s.persistentvolumeclaim.uid"
	pvcMetricNamespace    = "ddk8s.persistentvolumeclaim.namespace"
	pvcMetricClusterID    = "ddk8s.persistentvolumeclaim.cluster.id"
	pvcMetricClusterName  = "ddk8s.persistentvolumeclaim.cluster.name"
	pvcMetricName         = "ddk8s.persistentvolumeclaim.name"
	pvcMetricPhase        = "ddk8s.persistentvolumeclaim.phase"
	pvcMetricAccessModes  = "ddk8s.persistentvolumeclaim.access_mode"
	pvcMetricPvName       = "ddk8s.persistentvolumeclaim.pv_name"
	pvcMetricStorageClass = "ddk8s.persistentvolumeclaim.storage_class"
	pvcMetricVolumeMode   = "ddk8s.persistentvolumeclaim.volume_mode"
	pvcMetricCreateTime   = "ddk8s.persistentvolumeclaim.create_time"
	pvcMetricLabels       = "ddk8s.persistentvolumeclaim.labels"
	pvcMetricAnnotations  = "ddk8s.persistentvolumeclaim.annotations"
	pvcMetricFinalizers   = "ddk8s.persistentvolumeclaim.finalizers"
	pvcMetricType         = "ddk8s.persistentvolumeclaim.type"
)

// GetOtlpExportReqFromDatadogPVCData converts Datadog persistent volume claim data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogPVCData(origin, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorPersistentVolumeClaim)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(pvcPayloadErrorMessage)
	}

	pvcs := ddReq.GetPersistentVolumeClaims()

	if len(pvcs) == 0 {
		log.Println("no pvcs found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(pvcPayloadErrorMessage)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, pvc := range pvcs {
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
		appendPVCMetrics(&scopeMetrics, resourceAttributes, metricAttributes, pvc, timestamp)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendPVCMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, pvc *processv1.PersistentVolumeClaim, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(pvcMetricCapacity)

	var volumeCapacity int64

	metadata := pvc.GetMetadata()

	if metadata != nil {
		resourceAttributes.PutStr(pvcMetricUID, metadata.GetUid())
		metricAttributes.PutStr(pvcMetricNamespace, metadata.GetNamespace())
		metricAttributes.PutStr(pvcMetricName, metadata.GetName())
		metricAttributes.PutStr(pvcMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(pvcMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(pvcMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))
		metricAttributes.PutStr(pvcMetricType, "PersistentVolumeClaim")
		phaseDetails := pvc.GetStatus()
		if phaseDetails != nil {
			metricAttributes.PutStr(pvcMetricPhase, phaseDetails.GetPhase())
			if accessModes := phaseDetails.GetAccessModes(); accessModes != nil {
				metricAttributes.PutStr(pvcMetricAccessModes, strings.Join(accessModes, ","))
			}
			capacityMap := phaseDetails.GetCapacity()
			volumeCapacity = capacityMap["storage"]
		}

		pvcSpec := pvc.GetSpec()
		if pvcSpec != nil {
			metricAttributes.PutStr(pvcMetricPvName, pvcSpec.GetVolumeName())
			metricAttributes.PutStr(pvcMetricStorageClass, pvcSpec.GetStorageClassName())
			metricAttributes.PutStr(pvcMetricVolumeMode, pvcSpec.GetVolumeMode())
		}

		metricAttributes.PutInt(pvcMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))
	}

	var dataPoints pmetric.NumberDataPointSlice
	gauge := scopeMetric.SetEmptyGauge()
	dataPoints = gauge.DataPoints()
	dp := dataPoints.AppendEmpty()

	dp.SetTimestamp(pcommon.Timestamp(timestamp))
	dp.SetIntValue(volumeCapacity)

	attributeMap := dp.Attributes()
	metricAttributes.CopyTo(attributeMap)
}

func setHostK8sAttributes(metricAttributes pcommon.Map, clusterName string, clusterID string) {
	metricAttributes.PutStr(pvcMetricClusterID, clusterID)
	metricAttributes.PutStr(pvcMetricClusterName, clusterName)
}
