package service

import (
	"fmt"
	processv1 "github.com/DataDog/agent-payload/v5/process"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"log"
	"strings"
)

// Constants for Service metrics
const (
	// Metrics
	serviceMetricPortCount = "ddk8s.service.port_count"
	// Attributes
	serviceMetricUID         = "ddk8s.service.uid"
	serviceMetricNamespace   = "ddk8s.service.namespace"
	serviceMetricClusterID   = "ddk8s.service.cluster.id"
	serviceMetricClusterName = "ddk8s.cluster.name"
	serviceMetricName        = "ddk8s.service.name"
	serviceMetricLabels      = "ddk8s.service.labels"
	serviceMetricAnnotations = "ddk8s.service.annotations"
	serviceMetricFinalizers  = "ddk8s.service.finalizers"
	serviceMetricType        = "ddk8s.service.type"
	serviceMetricClusterIP   = "ddk8s.service.cluster_ip"
	serviceMetricPortsList   = "ddk8s.service.ports_list"
	serviceMetricCreateTime  = "ddk8s.service.create_time"
	// Error
	ErrNoMetricsInPayload = "No metrics related to Services found in Payload"
)

// GetOtlpExportReqFromDatadogServiceData converts Datadog Service data into OTLP ExportRequest.
func GetOtlpExportReqFromDatadogServiceData(origin string, key string, Body interface{}, timestamp int64) (pmetricotlp.ExportRequest, error) {
	ddReq, ok := Body.(*processv1.CollectorService)
	if !ok {
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(ErrNoMetricsInPayload)
	}
	services := ddReq.GetServices()

	if len(services) == 0 {
		log.Println("no services found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(ErrNoMetricsInPayload)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	clusterName := ddReq.GetClusterName()
	clusterID := ddReq.GetClusterId()

	for _, service := range services {
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
		appendServiceMetrics(&scopeMetrics, resourceAttributes, metricAttributes, service, timestamp)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func appendServiceMetrics(scopeMetrics *pmetric.ScopeMetrics, resourceAttributes pcommon.Map, metricAttributes pcommon.Map, service *processv1.Service, timestamp int64) {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(serviceMetricPortCount)

	var metricVal int64

	metadata := service.GetMetadata()
	if metadata != nil {
		resourceAttributes.PutStr(serviceMetricUID, metadata.GetUid())
		metricAttributes.PutStr(serviceMetricNamespace, metadata.GetNamespace())
		metricAttributes.PutStr(serviceMetricName, metadata.GetName())
		metricAttributes.PutStr(serviceMetricLabels, strings.Join(metadata.GetLabels(), "&"))
		metricAttributes.PutStr(serviceMetricAnnotations, strings.Join(metadata.GetAnnotations(), "&"))
		metricAttributes.PutStr(serviceMetricFinalizers, strings.Join(metadata.GetFinalizers(), ","))
	}

	specDetails := service.GetSpec()
	metricVal = int64(len(specDetails.GetPorts()))
	metricAttributes.PutStr(serviceMetricType, specDetails.GetType())
	metricAttributes.PutStr(serviceMetricClusterIP, specDetails.GetClusterIP())
	metricAttributes.PutStr(serviceMetricPortsList, convertPortRulesToString(specDetails.GetPorts()))
	metricAttributes.PutInt(serviceMetricCreateTime, helpers.CalculateCreateTime(metadata.GetCreationTimestamp()))

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
	metricAttributes.PutStr(serviceMetricClusterID, clusterID)
	metricAttributes.PutStr(serviceMetricClusterName, clusterName)
}

func convertPortRulesToString(serviceports []*processv1.ServicePort) string {
	var result strings.Builder

	for i, sp := range serviceports {
		if i > 0 {
			result.WriteString("&")
		}
		portString := fmt.Sprintf("%s %d/%s", sp.GetName(), sp.GetPort(), sp.GetProtocol())
		result.WriteString(portString)
	}

	return result.String()
}
