// Copyright The OpenTelemetry Authors
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

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	cinfo "github.com/google/cadvisor/info/v1"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

type ProcessesMetricExtractor struct {
	logger *zap.Logger
}

func (f *ProcessesMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasProcesses
}

func (f *ProcessesMetricExtractor) GetValue(info *cinfo.ContainerInfo, _ CPUMemInfoProvider, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	if containerType != ci.TypeContainer {
		return metrics
	}

	metric := newCadvisorMetric(containerType, f.logger)
	metric.cgroupPath = info.Name

	stats := GetStats(info)
	metric.fields[ci.MetricName(containerType, ci.Processes)] = stats.Processes.ProcessCount
	metric.fields[ci.MetricName(containerType, ci.ProcessesThreads)] = stats.Processes.ThreadsCurrent
	metric.fields[ci.MetricName(containerType, ci.ProcessesFileDescriptors)] = stats.Processes.FdCount

	metrics = append(metrics, metric)
	return metrics
}

func NewProcessesMetricExtractor(logger *zap.Logger) *ProcessesMetricExtractor {
	me := &ProcessesMetricExtractor{
		logger: logger,
	}

	return me
}
