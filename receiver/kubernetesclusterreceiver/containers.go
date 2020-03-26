// Copyright 2020 OpenTelemetry Authors
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

package kubernetesclusterreceiver

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opencensus.io/resource/resourcekeys"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubernetesclusterreceiver/utils"
)

const (
	// Keys for container properties.
	containerKeyStatus       = "container.status"
	containerKeyStatusReason = "container.status.reason"
)

var containerRestartMetric = &metricspb.MetricDescriptor{
	Name: "kubernetes/container/restarts",
	Description: "How many times the container has restarted in the recent past. " +
		"This value is pulled directly from the K8s API and the value can go indefinitely high" +
		" and be reset to 0 at any time depending on how your kubelet is configured to prune" +
		" dead containers. It is best to not depend too much on the exact value but rather look" +
		" at it as either == 0, in which case you can conclude there were no restarts in the recent" +
		" past, or > 0, in which case you can conclude there were restarts in the recent past, and" +
		" not try and analyze the value beyond that.",
	Unit: "1",
	Type: metricspb.MetricDescriptor_GAUGE_INT64,
}

var containerReadyMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/container/ready",
	Description: "Whether a container has passed its readiness probe (0 for no, 1 for yes)",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

// getStatusMetricsForContainer returns metrics about the status of the container.
func getStatusMetricsForContainer(cs corev1.ContainerStatus) []*metricspb.Metric {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: containerRestartMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(cs.RestartCount)),
			},
		},
		{
			MetricDescriptor: containerReadyMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(boolToInt(cs.Ready))),
			},
		},
	}

	return metrics

}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

var containerCPURequestMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/container/cpu_request",
	Description: "CPU requested for the container",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var containerCPULimitMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/container/cpu_limit",
	Description: "Maximum CPU limit set for the container",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var containerMemoryRequestMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/container/memory_request",
	Description: "Memory requested for the container",
	Unit:        "By",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var containerMemoryLimitMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/container/memory_limit",
	Description: "Maximum memory limit set for the container",
	Unit:        "By",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var containerEpemStorageRequestMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/container/ephemeral_storage_request",
	Description: "Ephemeral storage requested for the container",
	Unit:        "By",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var containerEpemStorageLimitMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/container/ephemeral_storage_limit",
	Description: "Maximum ephemeral storage limit set for the container",
	Unit:        "By",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var containerRequestMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/container/request",
	Description: "Resource requested for the container",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
	LabelKeys:   []*metricspb.LabelKey{{Key: "resource"}},
}

var containerLimitMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/container/limit",
	Description: "Maximum resource limit set for the container",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
	LabelKeys:   []*metricspb.LabelKey{{Key: "resource"}},
}

// getSpecMetricsForContainer metricizes values from the container spec.
// This includes values like resource requests and limits.
func getSpecMetricsForContainer(c corev1.Container) []*metricspb.Metric {
	metrics := make([]*metricspb.Metric, 0)

	if val, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
		metrics = append(metrics, []*metricspb.Metric{
			{
				MetricDescriptor: containerCPURequestMetric,
				Timeseries: []*metricspb.TimeSeries{
					utils.GetInt64TimeSeries(val.MilliValue()),
				},
			},
		}...)
	}

	if val, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
		metrics = append(metrics, []*metricspb.Metric{
			{
				MetricDescriptor: containerCPULimitMetric,
				Timeseries: []*metricspb.TimeSeries{
					utils.GetInt64TimeSeries(val.MilliValue()),
				},
			},
		}...)
	}

	if val, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
		metrics = append(metrics, []*metricspb.Metric{
			{
				MetricDescriptor: containerMemoryRequestMetric,
				Timeseries: []*metricspb.TimeSeries{
					utils.GetInt64TimeSeries(val.Value()),
				},
			},
		}...)
	}

	if val, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
		metrics = append(metrics, []*metricspb.Metric{
			{
				MetricDescriptor: containerMemoryLimitMetric,
				Timeseries: []*metricspb.TimeSeries{
					utils.GetInt64TimeSeries(val.Value()),
				},
			},
		}...)
	}

	if val, ok := c.Resources.Requests[corev1.ResourceEphemeralStorage]; ok {
		metrics = append(metrics, []*metricspb.Metric{
			{
				MetricDescriptor: containerEpemStorageRequestMetric,
				Timeseries: []*metricspb.TimeSeries{
					utils.GetInt64TimeSeries(val.Value()),
				},
			},
		}...)
	}

	if val, ok := c.Resources.Limits[corev1.ResourceEphemeralStorage]; ok {
		metrics = append(metrics, []*metricspb.Metric{
			{
				MetricDescriptor: containerEpemStorageLimitMetric,
				Timeseries: []*metricspb.TimeSeries{
					utils.GetInt64TimeSeries(val.Value()),
				},
			},
		}...)
	}

	// TODO: Choose between this and the above ways of capturing this information
	// The below seems concise since part of the information (resource type) is
	// captured as a label on the metric. However, it makes it difficult to handle
	// units since it changes based on resource type.

	for _, t := range []struct {
		metric *metricspb.MetricDescriptor
		rl     corev1.ResourceList
	}{
		{
			resourceQuotaHardLimitMetric,
			c.Resources.Requests,
		},
		{
			resourceQuotaUsedMetric,
			c.Resources.Limits,
		},
	} {
		for k, v := range t.rl {
			val := v.Value()
			if k == corev1.ResourceCPU {
				val = v.MilliValue()
			}

			metrics = append(metrics,
				&metricspb.Metric{
					MetricDescriptor: t.metric,
					Timeseries: []*metricspb.TimeSeries{
						utils.GetInt64TimeSeriesWithLabels(val, []*metricspb.LabelValue{{Value: string(k)}}),
					},
				},
			)
		}
	}

	return metrics

}

// getResourceForContainer returns a proto representation of the pod.
func getResourceForContainer(labels map[string]string) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type:   resourcekeys.ContainerType,
		Labels: labels,
	}
}

// getAllContainerLabels returns all container labels, including ones from
// the pod in which the container is running.
func getAllContainerLabels(id string, name string, image string,
	dims map[string]string) map[string]string {

	out := utils.CloneStringMap(dims)

	out[containerKeyID] = utils.StripContainerID(id)
	out[containerKeySpecName] = name
	out[resourcekeys.ContainerKeyImageName] = image

	return out
}

func getMetadataForContainer(cs corev1.ContainerStatus) *KubernetesMetadata {
	properties := make(map[string]string)

	if cs.State.Running != nil {
		properties[containerKeyStatus] = "running"
	}

	if cs.State.Terminated != nil {
		properties[containerKeyStatus] = "terminated"
		properties[containerKeyStatusReason] = cs.State.Terminated.Reason
	}

	if cs.State.Waiting != nil {
		properties[containerKeyStatus] = "waiting"
		properties[containerKeyStatusReason] = cs.State.Waiting.Reason
	}

	return &KubernetesMetadata{
		resourceIDKey: containerKeyID,
		resourceID:    utils.StripContainerID(cs.ContainerID),
		properties:    properties,
	}
}
