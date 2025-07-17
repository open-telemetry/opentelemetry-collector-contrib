// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	"fmt"
	"time"

	cinfo "github.com/google/cadvisor/info/v1"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
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
	GetValue(info *cinfo.ContainerInfo, mInfo CPUMemInfoProvider, containerType string) []*CAdvisorMetric
	Shutdown() error
}

type CAdvisorMetric struct {
	// source of the metric for debugging merge conflict
	cgroupPath string
	// key/value pairs that are typed and contain the metric (numerical) data
	fields map[string]any
	// key/value string pairs that are used to identify the metrics
	tags map[string]string

	logger *zap.Logger
}

func newCadvisorMetric(mType string, logger *zap.Logger) *CAdvisorMetric {
	metric := &CAdvisorMetric{
		fields: make(map[string]any),
		tags:   make(map[string]string),
		logger: logger,
	}
	metric.tags[ci.MetricType] = mType
	return metric
}

func (c *CAdvisorMetric) GetTags() map[string]string {
	return c.tags
}

func (c *CAdvisorMetric) GetFields() map[string]any {
	return c.fields
}

func (c *CAdvisorMetric) GetMetricType() string {
	return c.tags[ci.MetricType]
}

func (c *CAdvisorMetric) AddTags(tags map[string]string) {
	for k, v := range tags {
		c.tags[k] = v
	}
}

func (c *CAdvisorMetric) HasField(key string) bool {
	return c.fields[key] != nil
}

func (c *CAdvisorMetric) AddField(key string, val any) {
	c.fields[key] = val
}

func (c *CAdvisorMetric) GetField(key string) any {
	return c.fields[key]
}

func (c *CAdvisorMetric) HasTag(key string) bool {
	return c.tags[key] != ""
}

func (c *CAdvisorMetric) AddTag(key, val string) {
	c.tags[key] = val
}

func (c *CAdvisorMetric) GetTag(key string) string {
	return c.tags[key]
}

func (c *CAdvisorMetric) RemoveTag(key string) {
	delete(c.tags, key)
}

func (c *CAdvisorMetric) Merge(src *CAdvisorMetric) {
	// If there is any conflict, keep the fields with earlier timestamp
	for k, v := range src.fields {
		if _, ok := c.fields[k]; ok {
			c.logger.Debug(fmt.Sprintf("metric being merged has conflict in fields, src: %v, dest: %v \n", *src, *c))
			c.logger.Debug("metric being merged has conflict in fields", zap.String("src", src.cgroupPath), zap.String("dest", c.cgroupPath))
			if c.tags[ci.Timestamp] < src.tags[ci.Timestamp] {
				continue
			}
		}
		c.fields[k] = v
	}
}

func newFloat64RateCalculator() awsmetrics.MetricCalculator {
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

func assignRateValueToField(rateCalculator *awsmetrics.MetricCalculator, fields map[string]any, metricName string,
	cinfoName string, curVal any, curTime time.Time, multiplier float64,
) {
	mKey := awsmetrics.NewKey(cinfoName+metricName, nil)
	if val, ok := rateCalculator.Calculate(mKey, curVal, curTime); ok {
		fields[metricName] = val.(float64) * multiplier
	}
}

// MergeMetrics merges an array of cadvisor metrics based on common metric keys
func MergeMetrics(metrics []*CAdvisorMetric) []*CAdvisorMetric {
	result := make([]*CAdvisorMetric, 0, len(metrics))
	metricMap := make(map[string]*CAdvisorMetric)
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
func getMetricKey(metric *CAdvisorMetric) string {
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
