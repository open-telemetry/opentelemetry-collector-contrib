// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	"time"

	cinfo "github.com/google/cadvisor/info/v1"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
)

type MemMetricExtractor struct {
	logger         *zap.Logger
	rateCalculator awsmetrics.MetricCalculator
}

func (m *MemMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasMemory
}

func (m *MemMetricExtractor) GetValue(info *cinfo.ContainerInfo, mInfo CPUMemInfoProvider, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	if containerType == ci.TypeInfraContainer {
		return metrics
	}

	metric := newCadvisorMetric(containerType, m.logger)
	metric.cgroupPath = info.Name
	curStats := GetStats(info)

	metric.fields[ci.MetricName(containerType, ci.MemUsage)] = curStats.Memory.Usage
	metric.fields[ci.MetricName(containerType, ci.MemCache)] = curStats.Memory.Cache
	metric.fields[ci.MetricName(containerType, ci.MemRss)] = curStats.Memory.RSS
	metric.fields[ci.MetricName(containerType, ci.MemMaxusage)] = curStats.Memory.MaxUsage
	metric.fields[ci.MetricName(containerType, ci.MemSwap)] = curStats.Memory.Swap
	metric.fields[ci.MetricName(containerType, ci.MemFailcnt)] = curStats.Memory.Failcnt
	metric.fields[ci.MetricName(containerType, ci.MemMappedfile)] = curStats.Memory.MappedFile
	metric.fields[ci.MetricName(containerType, ci.MemWorkingset)] = curStats.Memory.WorkingSet

	multiplier := float64(time.Second)
	assignRateValueToField(&m.rateCalculator, metric.fields, ci.MetricName(containerType, ci.MemPgfault), info.Name,
		float64(curStats.Memory.ContainerData.Pgfault), curStats.Timestamp, multiplier)
	assignRateValueToField(&m.rateCalculator, metric.fields, ci.MetricName(containerType, ci.MemPgmajfault), info.Name,
		float64(curStats.Memory.ContainerData.Pgmajfault), curStats.Timestamp, multiplier)
	assignRateValueToField(&m.rateCalculator, metric.fields, ci.MetricName(containerType, ci.MemHierarchicalPgfault), info.Name,
		float64(curStats.Memory.HierarchicalData.Pgfault), curStats.Timestamp, multiplier)
	assignRateValueToField(&m.rateCalculator, metric.fields, ci.MetricName(containerType, ci.MemHierarchicalPgmajfault), info.Name,
		float64(curStats.Memory.HierarchicalData.Pgmajfault), curStats.Timestamp, multiplier)

	memoryCapacity := mInfo.GetMemoryCapacity()
	if metric.fields[ci.MetricName(containerType, ci.MemWorkingset)] != nil && memoryCapacity != 0 {
		metric.fields[ci.MetricName(containerType, ci.MemUtilization)] = float64(metric.fields[ci.MetricName(containerType, ci.MemWorkingset)].(uint64)) / float64(memoryCapacity) * 100
	}

	if containerType == ci.TypeNode || containerType == ci.TypeInstance {
		metric.fields[ci.MetricName(containerType, ci.MemLimit)] = memoryCapacity
	}

	metrics = append(metrics, metric)
	return metrics
}

func (m *MemMetricExtractor) Shutdown() error {
	return m.rateCalculator.Shutdown()
}

func NewMemMetricExtractor(logger *zap.Logger) *MemMetricExtractor {
	return &MemMetricExtractor{
		logger:         logger,
		rateCalculator: newFloat64RateCalculator(),
	}
}
