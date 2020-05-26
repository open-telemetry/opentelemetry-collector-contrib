// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubelet

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

type metricDataAccumulator struct {
	m []*consumerdata.MetricsData
}

const (
	k8sPrefix       = "k8s/"
	nodePrefix      = k8sPrefix + "node/"
	podPrefix       = k8sPrefix + "pod/"
	containerPrefix = k8sPrefix + "container/"
)

func (a *metricDataAccumulator) nodeStats(s stats.NodeStats) {
	// todo s.Runtime.ImageFs
	a.accumulate(
		timestampProto(s.StartTime.Time),
		nodeResource(s),

		cpuMetrics(nodePrefix, s.CPU),
		fsMetrics(nodePrefix, s.Fs),
		memMetrics(nodePrefix, s.Memory),
		networkMetrics(nodePrefix, s.Network),
	)
}

func (a *metricDataAccumulator) podStats(podResource *resourcepb.Resource, s stats.PodStats) {
	a.accumulate(
		timestampProto(s.StartTime.Time),
		podResource,

		cpuMetrics(podPrefix, s.CPU),
		fsMetrics(podPrefix, s.EphemeralStorage),
		memMetrics(podPrefix, s.Memory),
		networkMetrics(podPrefix, s.Network),
	)
}

func (a *metricDataAccumulator) containerStats(podResource *resourcepb.Resource, s stats.ContainerStats) {
	// todo s.Logs
	a.accumulate(
		timestampProto(s.StartTime.Time),
		containerResource(podResource, s),

		cpuMetrics(containerPrefix, s.CPU),
		memMetrics(containerPrefix, s.Memory),
		fsMetrics(containerPrefix, s.Rootfs),
	)
}

func (a *metricDataAccumulator) accumulate(
	startTime *timestamp.Timestamp,
	r *resourcepb.Resource,
	m ...[]*metricspb.Metric,
) {
	var resourceMetrics []*metricspb.Metric
	for _, metrics := range m {
		for _, metric := range metrics {
			if metric != nil {
				metric.Timeseries[0].StartTimestamp = startTime
				resourceMetrics = append(resourceMetrics, metric)
			}
		}
	}
	a.m = append(a.m, &consumerdata.MetricsData{
		Resource: r,
		Metrics:  resourceMetrics,
	})
}
