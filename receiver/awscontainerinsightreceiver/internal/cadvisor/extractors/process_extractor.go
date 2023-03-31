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

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	cinfo "github.com/google/cadvisor/info/v1"
	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"go.uber.org/zap"
)

type ProcessMetricExtractor struct {
	logger *zap.Logger
}

func (p *ProcessMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasProcesses
}

func (p *ProcessMetricExtractor) GetValue(info *cinfo.ContainerInfo, _ CPUMemInfoProvider, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric
	if containerType == ci.TypeInfraContainer {
		return metrics
	}

	metric := newCadvisorMetric(containerType, p.logger)
	curStats := GetStats(info)
	/* Getting the processes stats from cadvisor
	https://github.com/google/cadvisor/blob/4bed263005957c711b1cc5b853bc012a17609a8f/info/v1/container.go#L909-L927
	* ProcessCount: Number of processes in the container
	* FdCount: Number of file descriptor opening in the container
	* SocketCount: Number of sockets
	* ThreadsCurrent: Number of threads currently in container
	* ThreadsMax: Maxium number of threads allowed in container
	*/
	metric.fields[ci.MetricName(containerType, ci.ProcessCount)] = curStats.Processes.ProcessCount
	metric.fields[ci.MetricName(containerType, ci.FileDescriptorCount)] = curStats.Processes.FdCount
	metric.fields[ci.MetricName(containerType, ci.SocketCount)] = curStats.Processes.SocketCount
	metric.fields[ci.MetricName(containerType, ci.ThreadsCurrent)] = curStats.Processes.ThreadsCurrent
	metric.fields[ci.MetricName(containerType, ci.ThreadsMax)] = curStats.Processes.ThreadsMax

	metrics = append(metrics, metric)
	return metrics
}

func NewProcessMetricExtractor(logger *zap.Logger) *ProcessMetricExtractor {
	return &ProcessMetricExtractor{
		logger: logger,
	}
}
