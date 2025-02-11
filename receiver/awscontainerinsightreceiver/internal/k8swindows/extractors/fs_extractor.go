// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/extractors"

import (
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
	cExtractor "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

type FileSystemMetricExtractor struct {
	logger         *zap.Logger
	rateCalculator awsmetrics.MetricCalculator
}

func (f *FileSystemMetricExtractor) HasValue(rawMetric RawMetric) bool {
	return !rawMetric.Time.IsZero()
}

func (f *FileSystemMetricExtractor) GetValue(rawMetric RawMetric, _ cExtractor.CPUMemInfoProvider, containerType string) []*stores.CIMetricImpl {
	if containerType == ci.TypePod {
		return nil
	}

	containerType = getFSMetricType(containerType, f.logger)
	metrics := make([]*stores.CIMetricImpl, 0, len(rawMetric.FileSystemStats))

	for _, v := range rawMetric.FileSystemStats {
		metric := stores.NewCIMetric(containerType, f.logger)

		metric.AddTag(ci.DiskDev, v.Device)
		metric.AddTag(ci.FSType, v.Type)

		metric.AddField(ci.MetricName(containerType, ci.FSUsage), v.UsedBytes)
		metric.AddField(ci.MetricName(containerType, ci.FSCapacity), v.CapacityBytes)
		metric.AddField(ci.MetricName(containerType, ci.FSAvailable), v.AvailableBytes)

		if v.CapacityBytes != 0 {
			metric.AddField(ci.MetricName(containerType, ci.FSUtilization), float64(v.UsedBytes)/float64(v.CapacityBytes)*100)
		}

		metrics = append(metrics, metric)
	}
	return metrics
}

func (f *FileSystemMetricExtractor) Shutdown() error {
	return f.rateCalculator.Shutdown()
}

func NewFileSystemMetricExtractor(logger *zap.Logger) *FileSystemMetricExtractor {
	return &FileSystemMetricExtractor{
		logger:         logger,
		rateCalculator: cExtractor.NewFloat64RateCalculator(),
	}
}

func getFSMetricType(containerType string, logger *zap.Logger) string {
	metricType := ""
	switch containerType {
	case ci.TypeNode:
		metricType = ci.TypeNodeFS
	case ci.TypeContainer:
		metricType = ci.TypeContainerFS
	default:
		logger.Warn("fs_extractor: fs metric extractor is parsing unexpected containerType", zap.String("containerType", containerType))
	}
	return metricType
}
