// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	"regexp"

	cinfo "github.com/google/cadvisor/info/v1"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

var allowedPaths = regexp.MustCompile(`^(tmpfs|\/dev\/.*|overlay)$`)

type FileSystemMetricExtractor struct {
	allowListRegexP *regexp.Regexp
	logger          *zap.Logger
}

func (f *FileSystemMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasFilesystem
}

func (f *FileSystemMetricExtractor) GetValue(info *cinfo.ContainerInfo, _ CPUMemInfoProvider, containerType string) []*CAdvisorMetric {
	if containerType == ci.TypePod || containerType == ci.TypeInfraContainer {
		return nil
	}

	containerType = getFSMetricType(containerType, f.logger)
	stats := GetStats(info)
	metrics := make([]*CAdvisorMetric, 0, len(stats.Filesystem))

	for _, v := range stats.Filesystem {
		metric := newCadvisorMetric(containerType, f.logger)
		if v.Device == "" {
			continue
		}
		if f.allowListRegexP != nil && !f.allowListRegexP.MatchString(v.Device) {
			continue
		}

		metric.tags[ci.DiskDev] = v.Device
		metric.tags[ci.FSType] = v.Type

		metric.fields[ci.MetricName(containerType, ci.FSUsage)] = v.Usage
		metric.fields[ci.MetricName(containerType, ci.FSCapacity)] = v.Limit
		metric.fields[ci.MetricName(containerType, ci.FSAvailable)] = v.Available

		if v.Limit != 0 {
			metric.fields[ci.MetricName(containerType, ci.FSUtilization)] = float64(v.Usage) / float64(v.Limit) * 100
		}

		if v.HasInodes {
			metric.fields[ci.MetricName(containerType, ci.FSInodes)] = v.Inodes
			metric.fields[ci.MetricName(containerType, ci.FSInodesfree)] = v.InodesFree
		}

		metric.cgroupPath = info.Name
		metrics = append(metrics, metric)
	}
	return metrics
}

func (f *FileSystemMetricExtractor) Shutdown() error {
	return nil
}

func NewFileSystemMetricExtractor(logger *zap.Logger) *FileSystemMetricExtractor {
	fse := &FileSystemMetricExtractor{
		logger:          logger,
		allowListRegexP: allowedPaths,
	}

	return fse
}

func getFSMetricType(containerType string, logger *zap.Logger) string {
	metricType := ""
	switch containerType {
	case ci.TypeNode:
		metricType = ci.TypeNodeFS
	case ci.TypeInstance:
		metricType = ci.TypeInstanceFS
	case ci.TypeContainer:
		metricType = ci.TypeContainerFS
	default:
		logger.Warn("fs_extractor: fs metric extractor is parsing unexpected containerType", zap.String("containerType", containerType))
	}
	return metricType
}
