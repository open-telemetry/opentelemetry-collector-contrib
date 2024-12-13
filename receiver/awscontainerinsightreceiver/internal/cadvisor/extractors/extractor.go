// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	"fmt"
	"time"

	cinfo "github.com/google/cadvisor/info/v1"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

func GetStats(info *cinfo.ContainerInfo) *cinfo.ContainerStats {
	if len(info.Stats) == 0 {
		return nil
	}
	// When there is more than one stats point, always use the last one
	return info.Stats[len(info.Stats)-1]
}

type CPUMemInfoProvider interface {
	GetNumCores() int64
	GetMemoryCapacity() int64
}

type MetricExtractor interface {
	HasValue(*cinfo.ContainerInfo) bool
	GetValue(info *cinfo.ContainerInfo, mInfo CPUMemInfoProvider, containerType string) []*stores.CIMetricImpl
	Shutdown() error
}

func NewFloat64RateCalculator() awsmetrics.MetricCalculator {
	return awsmetrics.NewMetricCalculator(func(prev *awsmetrics.MetricValue, val any, timestamp time.Time) (any, bool) {
		if prev != nil {
			deltaNs := timestamp.Sub(prev.Timestamp)
			deltaValue := val.(float64) - prev.RawValue.(float64)
			if deltaNs > ci.MinTimeDiff && deltaValue >= 0 {
				return deltaValue / float64(deltaNs), true
			}
		}
		return float64(0), false
	})
}

func AssignRateValueToField(rateCalculator *awsmetrics.MetricCalculator, fields map[string]any, metricName string,
	cinfoName string, curVal any, curTime time.Time, multiplier float64,
) {
	mKey := awsmetrics.NewKey(cinfoName+metricName, nil)
	if val, ok := rateCalculator.Calculate(mKey, curVal, curTime); ok {
		fields[metricName] = val.(float64) * multiplier
	}
}

// MergeMetrics merges an array of cadvisor metrics based on common metric keys
func MergeMetrics(metrics []*stores.CIMetricImpl) []*stores.CIMetricImpl {
	result := make([]*stores.CIMetricImpl, 0, len(metrics))
	metricMap := make(map[string]*stores.CIMetricImpl)
	for _, metric := range metrics {
		if metricKey := getMetricKey(metric); metricKey != "" {
			if mergedMetric, ok := metricMap[metricKey]; ok {
				mergedMetric.Merge(metric)
			} else {
				metricMap[metricKey] = metric
			}
		} else {
			// this metric cannot be merged
			result = append(result, metric)
		}
	}
	for _, metric := range metricMap {
		result = append(result, metric)
	}
	return result
}

// return MetricKey for merge-able metrics
func getMetricKey(metric *stores.CIMetricImpl) string {
	metricType := metric.GetMetricType()
	var metricKey string
	switch metricType {
	case ci.TypeInstance:
		// merge cpu, memory, net metric for type Instance
		metricKey = "metricType:" + ci.TypeInstance
	case ci.TypeNode:
		// merge cpu, memory, net metric for type Node
		metricKey = "metricType:" + ci.TypeNode
	case ci.TypePod:
		// merge cpu, memory, net metric for type Pod
		metricKey = fmt.Sprintf("metricType:%s,podId:%s", ci.TypePod, metric.GetTags()[ci.PodIDKey])
	case ci.TypeContainer:
		// merge cpu, memory metric for type Container
		metricKey = fmt.Sprintf("metricType:%s,podId:%s,containerName:%s", ci.TypeContainer, metric.GetTags()[ci.PodIDKey], metric.GetTags()[ci.ContainerNamekey])
	case ci.TypeInstanceDiskIO:
		// merge io_serviced, io_service_bytes for type InstanceDiskIO
		metricKey = fmt.Sprintf("metricType:%s,device:%s", ci.TypeInstanceDiskIO, metric.GetTags()[ci.DiskDev])
	case ci.TypeNodeDiskIO:
		// merge io_serviced, io_service_bytes for type NodeDiskIO
		metricKey = fmt.Sprintf("metricType:%s,device:%s", ci.TypeNodeDiskIO, metric.GetTags()[ci.DiskDev])
	default:
		metricKey = ""
	}
	return metricKey
}
