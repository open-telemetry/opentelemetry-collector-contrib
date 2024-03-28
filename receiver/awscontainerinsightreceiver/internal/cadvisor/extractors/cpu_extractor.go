// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	cInfo "github.com/google/cadvisor/info/v1"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

const (
	decimalToMillicores = 1000
)

type CPUMetricExtractor struct {
	logger         *zap.Logger
	rateCalculator awsmetrics.MetricCalculator
}

func (c *CPUMetricExtractor) HasValue(info *cInfo.ContainerInfo) bool {
	return info.Spec.HasCpu
}

func (c *CPUMetricExtractor) GetValue(info *cInfo.ContainerInfo, mInfo CPUMemInfoProvider, containerType string) []*stores.CIMetricImpl {
	var metrics []*stores.CIMetricImpl
	// Skip infra container and handle node, pod, other containers in pod
	if containerType == ci.TypeInfraContainer {
		return metrics
	}

	// When there is more than one stats point, always use the last one
	curStats := GetStats(info)
	metric := stores.NewCIMetric(containerType, c.logger)
	metric.ContainerName = info.Name
	multiplier := float64(decimalToMillicores)

	AssignRateValueToField(&c.rateCalculator, metric.Fields, ci.MetricName(containerType, ci.CPUTotal), info.Name, float64(curStats.Cpu.Usage.Total), curStats.Timestamp, multiplier)
	AssignRateValueToField(&c.rateCalculator, metric.Fields, ci.MetricName(containerType, ci.CPUUser), info.Name, float64(curStats.Cpu.Usage.User), curStats.Timestamp, multiplier)
	AssignRateValueToField(&c.rateCalculator, metric.Fields, ci.MetricName(containerType, ci.CPUSystem), info.Name, float64(curStats.Cpu.Usage.System), curStats.Timestamp, multiplier)

	numCores := mInfo.GetNumCores()
	if metric.Fields[ci.MetricName(containerType, ci.CPUTotal)] != nil && numCores != 0 {
		metric.Fields[ci.MetricName(containerType, ci.CPUUtilization)] = metric.Fields[ci.MetricName(containerType, ci.CPUTotal)].(float64) / float64(numCores*decimalToMillicores) * 100
	}

	if containerType == ci.TypeNode || containerType == ci.TypeInstance {
		metric.Fields[ci.MetricName(containerType, ci.CPULimit)] = numCores * decimalToMillicores
	}

	metrics = append(metrics, metric)
	return metrics
}

func (c *CPUMetricExtractor) Shutdown() error {
	return c.rateCalculator.Shutdown()
}

func NewCPUMetricExtractor(logger *zap.Logger) *CPUMetricExtractor {
	return &CPUMetricExtractor{
		logger:         logger,
		rateCalculator: NewFloat64RateCalculator(),
	}
}
