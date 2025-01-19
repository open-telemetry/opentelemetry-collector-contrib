// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	"fmt"
	"strings"
	"time"

	cInfo "github.com/google/cadvisor/info/v1"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
)

type DiskIOMetricExtractor struct {
	logger         *zap.Logger
	rateCalculator awsmetrics.MetricCalculator
}

func (d *DiskIOMetricExtractor) HasValue(info *cInfo.ContainerInfo) bool {
	return info.Spec.HasDiskIo
}

func (d *DiskIOMetricExtractor) GetValue(info *cInfo.ContainerInfo, _ CPUMemInfoProvider, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	if containerType != ci.TypeNode && containerType != ci.TypeInstance {
		return metrics
	}

	curStats := GetStats(info)
	metrics = append(metrics, d.extractIoMetrics(curStats.DiskIo.IoServiceBytes, ci.DiskIOServiceBytesPrefix, containerType, info.Name, curStats.Timestamp)...)
	metrics = append(metrics, d.extractIoMetrics(curStats.DiskIo.IoServiced, ci.DiskIOServicedPrefix, containerType, info.Name, curStats.Timestamp)...)
	return metrics
}

func (d *DiskIOMetricExtractor) extractIoMetrics(curStatsSet []cInfo.PerDiskStats, namePrefix string, containerType string, infoName string, curTime time.Time) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	expectedKey := []string{ci.DiskIOAsync, ci.DiskIOSync, ci.DiskIORead, ci.DiskIOWrite, ci.DiskIOTotal}
	for _, cur := range curStatsSet {
		curDevName := devName(cur)
		metric := newCadvisorMetric(getDiskIOMetricType(containerType, d.logger), d.logger)
		metric.tags[ci.DiskDev] = curDevName
		for _, key := range expectedKey {
			if curVal, curOk := cur.Stats[key]; curOk {
				mname := ci.MetricName(containerType, ioMetricName(namePrefix, key))
				assignRateValueToField(&d.rateCalculator, metric.fields, mname, infoName, float64(curVal), curTime, float64(time.Second))
			}
		}
		if len(metric.fields) > 0 {
			metrics = append(metrics, metric)
		}
	}
	return metrics
}

func (d *DiskIOMetricExtractor) Shutdown() error {
	return d.rateCalculator.Shutdown()
}

func ioMetricName(prefix, key string) string {
	return prefix + strings.ToLower(key)
}

func devName(dStats cInfo.PerDiskStats) string {
	devName := dStats.Device
	if devName == "" {
		devName = fmt.Sprintf("%d:%d", dStats.Major, dStats.Minor)
	}
	return devName
}

func NewDiskIOMetricExtractor(logger *zap.Logger) *DiskIOMetricExtractor {
	return &DiskIOMetricExtractor{
		logger:         logger,
		rateCalculator: newFloat64RateCalculator(),
	}
}

func getDiskIOMetricType(containerType string, logger *zap.Logger) string {
	metricType := ""
	switch containerType {
	case ci.TypeNode:
		metricType = ci.TypeNodeDiskIO
	case ci.TypeInstance:
		metricType = ci.TypeInstanceDiskIO
	case ci.TypeContainer:
		metricType = ci.TypeContainerDiskIO
	default:
		logger.Warn("diskio_extractor: diskIO metric extractor is parsing unexpected containerType", zap.String("containerType", containerType))
	}
	return metricType
}
