// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package containerinsight // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// SumFields takes an array of type map[string]interface{} and do
// the summation on the values corresponding to the same keys.
// It is assumed that the underlying type of interface{} to be float64.
func SumFields(fields []map[string]interface{}) map[string]float64 {
	if len(fields) == 0 {
		return nil
	}

	result := make(map[string]float64)
	// Use the first element as the base
	for k, v := range fields[0] {
		if fv, ok := v.(float64); ok {
			result[k] = fv
		}
	}

	if len(fields) == 1 {
		return result
	}

	for i := 1; i < len(fields); i++ {
		for k, v := range result {
			if fields[i][k] == nil {
				continue
			}
			if fv, ok := fields[i][k].(float64); ok {
				result[k] = v + fv
			}
		}
	}
	return result
}

// IsNode checks if a type belongs to node level metrics (for EKS)
func IsNode(mType string) bool {
	switch mType {
	case TypeNode, TypeNodeNet, TypeNodeFS, TypeNodeDiskIO:
		return true
	}
	return false
}

// IsInstance checks if a type belongs to instance level metrics (for ECS)
func IsInstance(mType string) bool {
	switch mType {
	case TypeInstance, TypeInstanceNet, TypeInstanceFS, TypeInstanceDiskIO:
		return true
	}
	return false
}

// IsContainer checks if a type belongs to container level metrics
func IsContainer(mType string) bool {
	switch mType {
	case TypeContainer, TypeContainerDiskIO, TypeContainerFS:
		return true
	}
	return false
}

// IsPod checks if a type belongs to container level metrics
func IsPod(mType string) bool {
	switch mType {
	case TypePod, TypePodNet:
		return true
	}
	return false
}

func getPrefixByMetricType(mType string) string {
	prefix := ""
	instancePrefix := "instance_"
	nodePrefix := "node_"
	instanceNetPrefix := "instance_interface_"
	nodeNetPrefix := "node_interface_"
	podPrefix := "pod_"
	podNetPrefix := "pod_interface_"
	containerPrefix := "container_"
	service := "service_"
	cluster := "cluster_"
	namespace := "namespace_"
	deployment := "deployment_"
	daemonSet := "daemonset_"

	switch mType {
	case TypeInstance:
		prefix = instancePrefix
	case TypeInstanceFS:
		prefix = instancePrefix
	case TypeInstanceDiskIO:
		prefix = instancePrefix
	case TypeInstanceNet:
		prefix = instanceNetPrefix
	case TypeNode:
		prefix = nodePrefix
	case TypeNodeFS:
		prefix = nodePrefix
	case TypeNodeDiskIO:
		prefix = nodePrefix
	case TypeNodeNet:
		prefix = nodeNetPrefix
	case TypePod:
		prefix = podPrefix
	case TypePodNet:
		prefix = podNetPrefix
	case TypeContainer:
		prefix = containerPrefix
	case TypeContainerDiskIO:
		prefix = containerPrefix
	case TypeContainerFS:
		prefix = containerPrefix
	case TypeService:
		prefix = service
	case TypeCluster:
		prefix = cluster
	case TypeClusterService:
		prefix = service
	case TypeClusterNamespace:
		prefix = namespace
	case TypeClusterDeployment:
		prefix = deployment
	case TypeClusterDaemonSet:
		prefix = daemonSet
	default:
		log.Printf("E! Unexpected MetricType: %s", mType)
	}
	return prefix
}

// MetricName returns the metric name based on metric type and measurement name
// For example, a type "node" and a measurement "cpu_utilization" gives "node_cpu_utilization"
func MetricName(mType string, measurement string) string {
	return getPrefixByMetricType(mType) + measurement
}

// RemovePrefix removes the prefix (e.g. "node_", "pod_") from the metric name
func RemovePrefix(mType string, metricName string) string {
	prefix := getPrefixByMetricType(mType)
	return strings.Replace(metricName, prefix, "", 1)
}

// GetUnitForMetric returns unit for a given metric
func GetUnitForMetric(metric string) string {
	return metricToUnitMap[metric]
}

// ConvertToOTLPMetrics converts a field containing metric values and a tag containing the relevant labels to OTLP metrics
func ConvertToOTLPMetrics(fields map[string]interface{}, tags map[string]string, logger *zap.Logger) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	var timestamp pcommon.Timestamp
	resource := rm.Resource()
	for tagKey, tagValue := range tags {
		if tagKey == Timestamp {
			timeNs, _ := strconv.ParseUint(tagValue, 10, 64)
			timestamp = pcommon.Timestamp(timeNs)
			// convert from nanosecond to millisecond (as emf log use millisecond timestamp)
			tagValue = strconv.FormatUint(timeNs/uint64(time.Millisecond), 10)
		}
		resource.Attributes().PutStr(tagKey, tagValue)
	}

	ilms := rm.ScopeMetrics()

	metricType := tags[MetricType]
	for key, value := range fields {
		metric := RemovePrefix(metricType, key)
		unit := GetUnitForMetric(metric)
		switch t := value.(type) {
		case int:
			intGauge(ilms.AppendEmpty(), key, unit, int64(t), timestamp)
		case int32:
			intGauge(ilms.AppendEmpty(), key, unit, int64(t), timestamp)
		case int64:
			intGauge(ilms.AppendEmpty(), key, unit, t, timestamp)
		case uint:
			intGauge(ilms.AppendEmpty(), key, unit, int64(t), timestamp)
		case uint32:
			intGauge(ilms.AppendEmpty(), key, unit, int64(t), timestamp)
		case uint64:
			intGauge(ilms.AppendEmpty(), key, unit, int64(t), timestamp)
		case float32:
			doubleGauge(ilms.AppendEmpty(), key, unit, float64(t), timestamp)
		case float64:
			doubleGauge(ilms.AppendEmpty(), key, unit, t, timestamp)
		default:
			valueType := fmt.Sprintf("%T", value)
			logger.Warn("Detected unexpected field", zap.String("key", key), zap.Any("value", value), zap.String("value type", valueType))
		}
	}

	return md
}

func intGauge(ilm pmetric.ScopeMetrics, metricName string, unit string, value int64, ts pcommon.Timestamp) {
	metric := initMetric(ilm, metricName, unit)

	intGauge := metric.SetEmptyGauge()
	dataPoints := intGauge.DataPoints()
	dataPoint := dataPoints.AppendEmpty()

	dataPoint.SetIntValue(value)
	dataPoint.SetTimestamp(ts)
}

func doubleGauge(ilm pmetric.ScopeMetrics, metricName string, unit string, value float64, ts pcommon.Timestamp) {
	metric := initMetric(ilm, metricName, unit)

	doubleGauge := metric.SetEmptyGauge()
	dataPoints := doubleGauge.DataPoints()
	dataPoint := dataPoints.AppendEmpty()

	dataPoint.SetDoubleValue(value)
	dataPoint.SetTimestamp(ts)
}

func initMetric(ilm pmetric.ScopeMetrics, name, unit string) pmetric.Metric {
	metric := ilm.Metrics().AppendEmpty()
	metric.SetName(name)
	metric.SetUnit(unit)

	return metric
}
