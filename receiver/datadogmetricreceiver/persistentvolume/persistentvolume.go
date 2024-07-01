package persistentvolume

import (
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Common constants
const (
	scopeName        = "mw"
	scopeVersion     = "v0.0.1"
	pvMetricCapacity = "ddk8s.persistentvolume.capacity"

	// Attributes
	pvAttrUID          = "ddk8s.persistentvolume.uid"
	pvAttrNamespace    = "ddk8s.persistentvolume.namespace"
	pvAttrName         = "ddk8s.persistentvolume.name"
	pvAttrLabels       = "ddk8s.persistentvolume.labels"
	pvAttrAnnotations  = "ddk8s.persistentvolume.annotations"
	pvAttrFinalizers   = "ddk8s.persistentvolume.finalizers"
	pvAttrType         = "ddk8s.persistentvolume.type"
	pvAttrPhase        = "ddk8s.persistentvolume.phase"
	pvAttrStorageClass = "ddk8s.persistentvolume.storage_class"
	pvAttrVolumeMode   = "ddk8s.persistentvolume.volume_mode"
	pvAttrAccessMode   = "ddk8s.persistentvolume.access_mode"
	pvAttrClaimPolicy  = "ddk8s.persistentvolume.claim_policy"
	pvAttrCreateTime   = "ddk8s.persistentvolume.create_time"
	pvAttrClusterID    = "ddk8s.persistentvolume.cluster.id"
	pvAttrClusterName  = "ddk8s.persistentvolume.cluster.name"
)

// GetOtlpExportReqFromDatadogPVData converts Datadog persistent volume data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogPVData(origin string, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorPersistentVolume)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload("No metrics related to PersistentVolumes found in Payload")
	}

	pvs := ddReq.GetPersistentVolumes()

	if len(pvs) == 0 {
		log.Println("no pvs found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload("No metrics related to PersistentVolumes found in Payload")
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, pv := range pvs {
		rm := resourceMetrics.AppendEmpty()
		resourceAttributes := rm.Resource().Attributes()

		commonResourceAttributes := helpers.CommonResourceAttributes{
			Origin:   origin,
			ApiKey:   key,
			MwSource: "datadog",
		}
		helpers.SetMetricResourceAttributes(resourceAttributes, commonResourceAttributes)

		scopeMetrics := helpers.AppendInstrScope(&rm)
		setHostK8sAttributes(resourceAttributes, clusterName, clusterID)
		appendPVMetrics(&scopeMetrics, resourceAttributes, pv, timestamp)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendPVMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, pv *processv1.PersistentVolume, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(pvMetricCapacity)

	metricAttributes := pcommon.NewMap()
	var volumeCapacity int64

	metadata := pv.GetMetadata()
	if metadata != nil {
		resourceAttributes.PutStr(pvAttrUID, metadata.GetUid())
		metricAttributes.PutStr(pvAttrNamespace, metadata.GetNamespace())
		metricAttributes.PutStr(pvAttrName, metadata.GetName())
		metricAttributes.PutStr(pvAttrLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(pvAttrAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(pvAttrFinalizers, strings.Join(metadata.GetFinalizers(), ","))
		metricAttributes.PutStr(pvAttrType, "PersistentVolume")

		phaseDetails := pv.GetStatus()
		if phaseDetails != nil {
			metricAttributes.PutStr(pvAttrPhase, phaseDetails.GetPhase())
		}

		pvcSpec := pv.GetSpec()
		if pvcSpec != nil {
			if capacityMap := pvcSpec.GetCapacity(); capacityMap != nil {
				volumeCapacity = capacityMap["storage"]
			}
			metricAttributes.PutStr(pvAttrStorageClass, pvcSpec.GetStorageClassName())
			metricAttributes.PutStr(pvAttrVolumeMode, pvcSpec.GetVolumeMode())
			if accessModes := pvcSpec.GetAccessModes(); accessModes != nil {
				metricAttributes.PutStr(pvAttrAccessMode, strings.Join(accessModes, ","))
			}
			metricAttributes.PutStr(pvAttrClaimPolicy, pvcSpec.GetPersistentVolumeReclaimPolicy())
		}
		metricAttributes.PutInt(pvAttrCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))
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

func setHostK8sAttributes(resourceAttributes pcommon.Map, clusterName string, clusterID string) {
	resourceAttributes.PutStr(pvAttrClusterID, clusterID)
	resourceAttributes.PutStr(pvAttrClusterName, clusterName)
}
