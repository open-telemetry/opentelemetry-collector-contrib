// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/extractors"

import (
	"time"

	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
	cExtractor "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

type MemMetricExtractor struct {
	logger         *zap.Logger
	rateCalculator awsmetrics.MetricCalculator
}

func (m *MemMetricExtractor) HasValue(rawMetric RawMetric) bool {
	return !rawMetric.Time.IsZero()
}

func (m *MemMetricExtractor) GetValue(rawMetric RawMetric, mInfo cExtractor.CPUMemInfoProvider, containerType string) []*stores.CIMetricImpl {
	var metrics []*stores.CIMetricImpl
	metric := stores.NewCIMetric(containerType, m.logger)
	identifier := rawMetric.Id

	metric.AddField(ci.MetricName(containerType, ci.MemUsage), rawMetric.MemoryStats.UsageBytes)
	metric.AddField(ci.MetricName(containerType, ci.MemRss), rawMetric.MemoryStats.RSSBytes)
	metric.AddField(ci.MetricName(containerType, ci.MemWorkingset), rawMetric.MemoryStats.WorkingSetBytes)

	multiplier := float64(time.Second)
	cExtractor.AssignRateValueToField(&m.rateCalculator, metric.GetFields(), ci.MetricName(containerType, ci.MemPgfault), identifier,
		float64(rawMetric.MemoryStats.PageFaults), rawMetric.Time, multiplier)
	cExtractor.AssignRateValueToField(&m.rateCalculator, metric.GetFields(), ci.MetricName(containerType, ci.MemPgmajfault), identifier,
		float64(rawMetric.MemoryStats.MajorPageFaults), rawMetric.Time, multiplier)

	memoryCapacity := mInfo.GetMemoryCapacity()
	if metric.GetField(ci.MetricName(containerType, ci.MemWorkingset)) != nil && memoryCapacity != 0 {
		metric.AddField(ci.MetricName(containerType, ci.MemUtilization), float64(metric.GetField(ci.MetricName(containerType, ci.MemWorkingset)).(uint64))/float64(memoryCapacity)*100)
	}

	if containerType == ci.TypeNode {
		metric.AddField(ci.MetricName(containerType, ci.MemLimit), memoryCapacity)
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
		rateCalculator: cExtractor.NewFloat64RateCalculator(),
	}
}
