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
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opentelemetry.io/collector/translator/conventions"
	appsv1 "k8s.io/api/apps/v1"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/utils"
)

const (
	// Keys for stateful set metadata.
	statefulSetCurrentVersion = "current_revision"
	statefulSetUpdateVersion  = "update_revision"
)

var statefulSetReplicasDesiredMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.statefulset.desired_pods",
	Description: "Number of desired pods in the stateful set (the `spec.replicas` field)",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var statefulSetReplicasReadyMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.statefulset.ready_pods",
	Description: "Number of pods created by the stateful set that have the `Ready` condition",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var statefulSetReplicasCurrentMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.statefulset.current_pods",
	Description: "The number of pods created by the StatefulSet controller from the StatefulSet version",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var statefulSetReplicasUpdatedMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.statefulset.updated_pods",
	Description: "Number of pods created by the StatefulSet controller from the StatefulSet version",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

func getMetricsForStatefulSet(ss *appsv1.StatefulSet) []*resourceMetrics {
	if ss.Spec.Replicas == nil {
		return []*resourceMetrics{}
	}

	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: statefulSetReplicasDesiredMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(*ss.Spec.Replicas)),
			},
		},
		{
			MetricDescriptor: statefulSetReplicasReadyMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(ss.Status.ReadyReplicas)),
			},
		},
		{
			MetricDescriptor: statefulSetReplicasCurrentMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(ss.Status.CurrentReplicas)),
			},
		},
		{
			MetricDescriptor: statefulSetReplicasUpdatedMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(ss.Status.UpdatedReplicas)),
			},
		},
	}

	return []*resourceMetrics{
		{
			resource: getResourceForStatefulSet(ss),
			metrics:  metrics,
		},
	}
}

func getResourceForStatefulSet(ss *appsv1.StatefulSet) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: k8sType,
		Labels: map[string]string{
			conventions.AttributeK8sStatefulSetUID: string(ss.UID),
			conventions.AttributeK8sStatefulSet:    ss.Name,
			conventions.AttributeK8sNamespace:      ss.Namespace,
			conventions.AttributeK8sCluster:        ss.ClusterName,
		},
	}
}

func getMetadataForStatefulSet(ss *appsv1.StatefulSet) map[metadata.ResourceID]*KubernetesMetadata {
	km := getGenericMetadata(&ss.ObjectMeta, k8sStatefulSet)
	km.metadata[statefulSetCurrentVersion] = ss.Status.CurrentRevision
	km.metadata[statefulSetUpdateVersion] = ss.Status.UpdateRevision

	return map[metadata.ResourceID]*KubernetesMetadata{metadata.ResourceID(ss.UID): km}
}
