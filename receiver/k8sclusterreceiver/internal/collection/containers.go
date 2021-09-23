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

package collection

import (
	"fmt"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testing/util"
	metadataPkg "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

const (
	// Keys for container metadata.
	containerKeyStatus       = "container.status"
	containerKeyStatusReason = "container.status.reason"

	// Values for container metadata
	containerStatusRunning    = "running"
	containerStatusWaiting    = "waiting"
	containerStatusTerminated = "terminated"
)

var containerRestartMetric = &metricspb.MetricDescriptor{
	Name: "k8s.container.restarts",
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
	Name:        "k8s.container.ready",
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
				utils.GetInt64TimeSeries(boolToInt64(cs.Ready)),
			},
		},
	}

	return metrics
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// getSpecMetricsForContainer metricizes values from the container spec.
// This includes values like resource requests and limits.
func getSpecMetricsForContainer(c corev1.Container) []*metricspb.Metric {
	metrics := make([]*metricspb.Metric, 0)

	for _, t := range []struct {
		typ         string
		description string
		rl          corev1.ResourceList
	}{
		{
			"request",
			"Resource requested for the container. " +
				"See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core for details",
			c.Resources.Requests,
		},
		{
			"limit",
			"Maximum resource limit set for the container. " +
				"See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core for details",
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
					MetricDescriptor: &metricspb.MetricDescriptor{
						Name:        fmt.Sprintf("k8s.container.%s_%s", k, t.typ),
						Description: t.description,
						Type:        metricspb.MetricDescriptor_GAUGE_INT64,
					},
					Timeseries: []*metricspb.TimeSeries{
						utils.GetInt64TimeSeries(val),
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
		Type:   containerType,
		Labels: labels,
	}
}

// getAllContainerLabels returns all container labels, including ones from
// the pod in which the container is running.
func getAllContainerLabels(cs corev1.ContainerStatus,
	dims map[string]string) map[string]string {

	out := util.CloneStringMap(dims)

	out[conventions.AttributeContainerID] = utils.StripContainerID(cs.ContainerID)
	out[conventions.AttributeK8SContainerName] = cs.Name
	out[conventions.AttributeContainerImageName] = cs.Image

	return out
}

func getMetadataForContainer(cs corev1.ContainerStatus) *KubernetesMetadata {
	metadata := map[string]string{}

	if cs.State.Running != nil {
		metadata[containerKeyStatus] = containerStatusRunning
	}

	if cs.State.Terminated != nil {
		metadata[containerKeyStatus] = containerStatusTerminated
		metadata[containerKeyStatusReason] = cs.State.Terminated.Reason
	}

	if cs.State.Waiting != nil {
		metadata[containerKeyStatus] = containerStatusWaiting
		metadata[containerKeyStatusReason] = cs.State.Waiting.Reason
	}

	return &KubernetesMetadata{
		resourceIDKey: conventions.AttributeContainerID,
		resourceID:    metadataPkg.ResourceID(utils.StripContainerID(cs.ContainerID)),
		metadata:      metadata,
	}
}
