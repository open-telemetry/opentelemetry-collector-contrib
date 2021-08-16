// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package extractors

import (
	"regexp"

	cinfo "github.com/google/cadvisor/info/v1"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

const (
	allowList = "^tmpfs$|^/dev/|^overlay$"
)

type FileSystemMetricExtractor struct {
	allowListRegexP *regexp.Regexp
	logger          *zap.Logger
}

func (f *FileSystemMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasFilesystem
}

func (f *FileSystemMetricExtractor) GetValue(info *cinfo.ContainerInfo, _ CPUMemInfoProvider, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	if containerType == ci.TypePod || info.Spec.Labels[containerNameLable] == infraContainerName {
		return metrics
	}

	containerType = getFSMetricType(containerType, f.logger)
	stats := GetStats(info)

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

		metrics = append(metrics, metric)
	}
	return metrics
}

func NewFileSystemMetricExtractor(logger *zap.Logger) *FileSystemMetricExtractor {
	fse := &FileSystemMetricExtractor{
		logger: logger,
	}
	if p, err := regexp.Compile(allowList); err == nil {
		fse.allowListRegexP = p
	} else {
		logger.Error("NewFileSystemMetricExtractor set regex failed", zap.Error(err))
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
