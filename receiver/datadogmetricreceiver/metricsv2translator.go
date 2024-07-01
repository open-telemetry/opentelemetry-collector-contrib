package datadogmetricreceiver

import (
	"log"
	"math"
	"strings"

	metricsV2 "github.com/DataDog/agent-payload/v5/gogen"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver/helpers"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
)

// MetricTranslator function type
type MetricTranslator func(*metricsV2.MetricPayload_MetricSeries, string, map[string]string, pcommon.Map, pcommon.Map) bool

type MetricBuilder func(*metricsV2.MetricPayload_MetricSeries, pmetric.Metric, pcommon.Map)

// Global variables for translators
var (
	translators = []MetricTranslator{
		translateKubernetesStateCountMetrics,
		translateContainerMetrics,
		translateKubernetesStatePod,
		translateKubernetesStateNode,
		translateKubernetes,
		translateKubernetesStateContainer,
	}
)

const (
	// Error
	ErrNoMetricsInSeriesPayload = "No metrics found in Payload V2 Series"
	// Suffix
	countSuffix = "count"
	totalSuffix = "total"
	// Prefix
	kubernetesStatePrefix          = "kubernetes_state"
	kubernetesStateNodePrefix      = "kubernetes_state.node"
	kubernetesStatePodPrefix       = "kubernetes_state.pod"
	kubernetesStateContainerPrefix = "kubernetes_state.container."
	systemCPUPrefix                = "system.cpu."
	kubernetesPrefix               = "kubernetes."
	containerPrefix                = "container."
	// Datadog Tags
	nodeTag          = "node"
	clusterNameTag   = "kube_cluster_name"
	namespaceTag     = "kube_namespace"
	containerIDTag   = "uid"
	podNameTag       = "pod_name"
	ddClusterNameTag = "dd_cluster_name"
	kubeServiceTag   = "kube_service"
	// Middleware Attribute Keys
	isCountKey       = "ddk8s.is_count"
	nodeNameKey      = "ddk8s.node.name"
	clusterNameKey   = "ddk8s.cluster.name"
	namespaceNameKey = "ddk8s.namespace.name"
	containerUIDKey  = "ddk8s.container.uid"
	podNameKey       = "ddk8s.pod.name"
	isKubeHost       = "ddk8s.is_kube_host"
	containerTagsKey = "ddk8s.container.tags"
	serviceNameKey   = "ddk8s.service.name"
)

// Main function to process Datadog metrics
func GetOtlpExportReqFromDatadogV2Metrics(origin, key string, ddReq metricsV2.MetricPayload) (pmetricotlp.ExportRequest, error) {

	if len(ddReq.GetSeries()) == 0 {
		log.Println("No Metrics found so skipping")
		return pmetricotlp.ExportRequest{}, helpers.NewErrNoMetricsInPayload(ErrNoMetricsInSeriesPayload)
	}

	metricHost := getMetricHost(ddReq.GetSeries())
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()

	for _, s := range ddReq.GetSeries() {
		//log.Println("s.GetMetric()", s.GetMetric())
		if helpers.SkipDatadogMetrics(s.GetMetric(), int32(s.GetType())) {
			continue
		}

		rm := resourceMetrics.AppendEmpty()
		resourceAttributes := rm.Resource().Attributes()
		tagMap := tagsToMap(s.GetTags())
		metricHost = getHostName(tagMap, metricHost)

		commonResourceAttributes := helpers.CommonResourceAttributes{
			Origin:   origin,
			ApiKey:   key,
			MwSource: "datadog",
			Host:     metricHost,
		}
		helpers.SetMetricResourceAttributes(resourceAttributes, commonResourceAttributes)

		scopeMetrics := helpers.AppendInstrScope(&rm)
		scopeMetric := initializeScopeMetric(s, &scopeMetrics)
		metricAttributes := pcommon.NewMap()
		// Currently This is added to Classify If A Metric is Datadog . Useful For Front Side
		metricAttributes.PutBool("datadog_metric", true)
		translateMetric(s, metricHost, tagMap, resourceAttributes, metricAttributes)
		setDataPoints(s, &scopeMetric, metricAttributes)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func tagsToMap(tags []string) map[string]string {
	tagMap := make(map[string]string)

	for _, tag := range tags {
		parts := strings.Split(tag, ":")
		if len(parts) == 2 {
			tagMap[parts[0]] = parts[1]
		}
	}

	return tagMap
}

func translateMetric(s *metricsV2.MetricPayload_MetricSeries, metricHost string, tagMap map[string]string, resourceAttributes, metricAttributes pcommon.Map) {
	for _, translator := range translators {
		if translator(s, metricHost, tagMap, resourceAttributes, metricAttributes) {
			return
		}
	}
	defaultTranslator(s, metricAttributes)
}

// extractHostName extracts the hostname if it's in the format HOSTNAME-CLUSTERNAME (for K8s screens)
// This is needed for middleware UI
func getHostName(tagMap map[string]string, metricHost string) string {
	if metricHost == "" {
		return ""
	}

	if clusterName := tagMap[ddClusterNameTag]; clusterName != "" {
		return extractHostName(metricHost, clusterName)
	}

	if clusterName := tagMap[clusterNameTag]; clusterName != "" {
		return extractHostName(metricHost, clusterName)
	}

	return metricHost
}

func extractHostName(metricHost string, clusterName string) string {
	strToMatch := "-" + clusterName
	idx := strings.LastIndex(metricHost, strToMatch)
	if idx != -1 {
		return metricHost[0:idx]
	}
	return metricHost
}

func getMetricHost(metricSeries []*metricsV2.MetricPayload_MetricSeries) string {
	host := ""
	for _, series := range metricSeries {

		// Iterate through each resource in the series
		for _, resource := range series.GetResources() {
			if resource.GetType() == "host" {
				host = resource.GetName()
			}
			// } else if i == 0 {
			// 	//resourceAttributes.PutStr(resource.GetType(), resource.GetName())
			// }
		}
		// Break the outer loop if metricHost is set
		if host != "" {
			break
		}
	}
	return host
}

func initializeScopeMetric(s *metricsV2.MetricPayload_MetricSeries, scopeMetrics *pmetric.ScopeMetrics) pmetric.Metric {
	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName(s.GetMetric())
	scopeMetric.SetUnit(s.GetUnit())
	return scopeMetric
}

func defaultTranslator(s *metricsV2.MetricPayload_MetricSeries, metricAttributes pcommon.Map) {
	// Decide if It is kubernetes Host
	// Used in Host Dialog Tabs
	if strings.Contains(s.GetMetric(), systemCPUPrefix) {
		metricAttributes.PutBool(isKubeHost, true)
	}
	for _, tag := range s.GetTags() {
		parts := strings.Split(tag, ":")
		if len(parts) == 2 {
			metricAttributes.PutStr(parts[0], parts[1])
		}
	}
}

func translateKubernetesStateCountMetrics(s *metricsV2.MetricPayload_MetricSeries, metricHost string, tagMap map[string]string, resourceAttributes, metricAttributes pcommon.Map) bool {
	metricName := s.GetMetric()
	if !strings.HasPrefix(metricName, kubernetesStatePrefix) {
		return false
	}

	if !strings.HasSuffix(metricName, countSuffix) && !strings.HasSuffix(metricName, totalSuffix) {
		return false
	}

	resourceAttributes.PutStr(isCountKey, "true")

	nodeName := tagMap[nodeTag]
	if nodeName == "" {
		nodeName = metricHost
	}
	resourceAttributes.PutStr(nodeNameKey, nodeName)

	metricAttributes.PutStr(clusterNameKey, tagMap[clusterNameTag])
	metricAttributes.PutStr(namespaceNameKey, tagMap[namespaceTag])

	for k, v := range tagMap {
		metricAttributes.PutStr(k, v)
	}

	return true
}

func translateContainerMetrics(s *metricsV2.MetricPayload_MetricSeries, metricHost string, tagMap map[string]string, resourceAttributes, metricAttributes pcommon.Map) bool {
	metricName := s.GetMetric()
	if !strings.HasPrefix(metricName, containerPrefix) {
		return false
	}

	//kubeClusterName := tagMap[clusterNameTag]
	containerID := tagMap["container_id"]

	if containerID == "" {
		return false
	}

	resourceAttributes.PutStr(containerUIDKey, containerID)
	resourceAttributes.PutStr(podNameKey, tagMap[podNameTag])
	// Note Assumption Node and Host Name are same
	nodeName := tagMap[nodeTag]
	if nodeName == "" {
		nodeName = metricHost
	}
	resourceAttributes.PutStr(nodeNameKey, nodeName)

	metricAttributes.PutStr(clusterNameKey, tagMap[ddClusterNameTag])
	if kubeNamespace, ok := tagMap[namespaceTag]; ok {
		metricAttributes.PutStr(namespaceNameKey, kubeNamespace)
	}
	metricAttributes.PutStr(containerTagsKey, strings.Join(s.GetTags(), "&"))

	if kubeService := tagMap[kubeServiceTag]; kubeService != "" {
		metricAttributes.PutStr(serviceNameKey, kubeService)
	}

	for k, v := range tagMap {
		metricAttributes.PutStr(k, v)
	}

	return true
}

func translateKubernetesStateNode(s *metricsV2.MetricPayload_MetricSeries, metricHost string, tagMap map[string]string, resourceAttributes, metricAttributes pcommon.Map) bool {
	metricName := s.GetMetric()
	if !strings.HasPrefix(metricName, kubernetesStateNodePrefix) {
		return false
	}
	// Note Assumption Node and Host Name are same
	nodeName := tagMap[nodeTag]
	if nodeName == "" {
		nodeName = metricHost
	}
	resourceAttributes.PutStr(nodeNameKey, nodeName)

	metricAttributes.PutStr(clusterNameKey, tagMap[clusterNameTag])
	metricAttributes.PutStr(namespaceNameKey, tagMap[namespaceTag])

	if kubeService := tagMap[kubeServiceTag]; kubeService != "" {
		metricAttributes.PutStr(serviceNameKey, kubeService)
	}

	for k, v := range tagMap {
		metricAttributes.PutStr(k, v)
	}

	return true
}

func translateKubernetesStatePod(s *metricsV2.MetricPayload_MetricSeries, metricHost string, tagMap map[string]string, resourceAttributes, metricAttributes pcommon.Map) bool {
	metricName := s.GetMetric()
	if !strings.HasPrefix(metricName, kubernetesStatePodPrefix) {
		return false
	}
	// Note Assumption Node and Host Name are same
	nodeName := tagMap[nodeTag]
	if nodeName == "" {
		nodeName = metricHost
	}
	resourceAttributes.PutStr(podNameKey, tagMap[podNameTag])
	resourceAttributes.PutStr(nodeNameKey, nodeName)
	metricAttributes.PutStr(clusterNameKey, tagMap[clusterNameTag])
	metricAttributes.PutStr(namespaceNameKey, tagMap[namespaceTag])

	if kubeService := tagMap[kubeServiceTag]; kubeService != "" {
		metricAttributes.PutStr(serviceNameKey, kubeService)
	}

	for k, v := range tagMap {
		if v == "" {
			continue
		}
		metricAttributes.PutStr(k, v)
	}

	return true
}

func translateKubernetes(s *metricsV2.MetricPayload_MetricSeries, metricHost string, tagMap map[string]string, resourceAttributes, metricAttributes pcommon.Map) bool {
	metricName := s.GetMetric()
	if !strings.HasPrefix(metricName, kubernetesPrefix) {
		return false
	}
	// Note Assumption Node and Host Name are same
	nodeName := tagMap[nodeTag]
	if nodeName == "" {
		nodeName = metricHost
	}
	resourceAttributes.PutStr(isCountKey, "true")

	metricAttributes.PutStr(namespaceNameKey, tagMap[namespaceTag])
	metricAttributes.PutStr(clusterNameKey, tagMap[clusterNameTag])

	if podName := tagMap[podNameTag]; podName != "" {
		resourceAttributes.PutStr(podNameKey, podName)
	}

	if nodeName := tagMap[nodeTag]; nodeName != "" {
		resourceAttributes.PutStr(nodeNameKey, nodeName)
	} else {
		resourceAttributes.PutStr(nodeNameKey, metricHost)
	}

	// Rewrite As It is Empty
	if tagMap[clusterNameTag] == "" {
		metricAttributes.PutStr(clusterNameKey, tagMap[ddClusterNameTag])
	}

	if kubeService := tagMap[kubeServiceTag]; kubeService != "" {
		metricAttributes.PutStr(serviceNameKey, kubeService)
	}

	for k, v := range tagMap {
		metricAttributes.PutStr(k, v)
	}

	return true
}

func translateKubernetesStateContainer(s *metricsV2.MetricPayload_MetricSeries, metricHost string, tagMap map[string]string, resourceAttributes, metricAttributes pcommon.Map) bool {
	metricName := s.GetMetric()
	if !strings.HasPrefix(metricName, kubernetesStateContainerPrefix) {
		return false
	}

	//kubeClusterName := tagMap[clusterNameTag]
	kubeNamespace := tagMap[namespaceTag]
	containerID := tagMap[containerIDTag]

	if containerID == "" {
		return false
	}

	resourceAttributes.PutStr(containerUIDKey, containerID)
	resourceAttributes.PutStr(podNameKey, tagMap[podNameTag])
	// Note Assumption Node and Host Name are same
	if nodeName := tagMap[nodeTag]; nodeName != "" {
		resourceAttributes.PutStr(nodeNameKey, nodeName)
	} else {
		resourceAttributes.PutStr(nodeNameKey, metricHost)
	}

	metricAttributes.PutStr(clusterNameKey, tagMap[clusterNameTag])
	metricAttributes.PutStr(namespaceNameKey, kubeNamespace)
	metricAttributes.PutStr(containerTagsKey, strings.Join(s.GetTags(), "&"))

	if kubeService := tagMap[kubeServiceTag]; kubeService != "" {
		metricAttributes.PutStr(serviceNameKey, kubeService)
	}

	for k, v := range tagMap {
		metricAttributes.PutStr(k, v)
	}

	return true
}

func setDataPoints(s *metricsV2.MetricPayload_MetricSeries, scopeMetric *pmetric.Metric, metricAttributes pcommon.Map) {
	var dataPoints pmetric.NumberDataPointSlice
	// in case datadog metric is rate, we need to multiply
	// the value in the metric by multiplyFactor to get the sum
	// for otlp metrics.
	multiplyFactor := 1.0
	switch s.GetType() {
	case metricsV2.MetricPayload_RATE:
		multiplyFactor = float64(s.GetInterval())
		fallthrough
	case metricsV2.MetricPayload_COUNT:
		sum := scopeMetric.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum.SetIsMonotonic(false)
		dataPoints = sum.DataPoints()
	case metricsV2.MetricPayload_GAUGE:
		gauge := scopeMetric.SetEmptyGauge()
		dataPoints = gauge.DataPoints()
	default:
		log.Println("datadog metric not yet handled", "type", s.Metric)
		return
	}

	for _, point := range s.GetPoints() {
		// Datadog payload stores timestamp as first member of Point array
		unixNano := float64(point.GetTimestamp()) * math.Pow(10, 9)
		dp := dataPoints.AppendEmpty()
		dp.SetTimestamp(pcommon.Timestamp(unixNano))
		// Datadog payload stores count value as second member of Point
		// array
		dp.SetDoubleValue(float64(point.GetValue()) * multiplyFactor)
		attributeMap := dp.Attributes()
		metricAttributes.CopyTo(attributeMap)
	}

}
