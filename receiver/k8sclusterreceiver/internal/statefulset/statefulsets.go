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

package statefulset // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/statefulset"

import (
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
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

func GetMetrics(ss *appsv1.StatefulSet) []*agentmetricspb.ExportMetricsServiceRequest {
	if ss.Spec.Replicas == nil {
		return []*agentmetricspb.ExportMetricsServiceRequest{}
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

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResource(ss),
			Metrics:  metrics,
		},
	}
}

func getResource(ss *appsv1.StatefulSet) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			conventions.AttributeK8SStatefulSetUID:  string(ss.UID),
			conventions.AttributeK8SStatefulSetName: ss.Name,
			conventions.AttributeK8SNamespaceName:   ss.Namespace,
		},
	}
}

func GetMetadata(ss *appsv1.StatefulSet) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	km := metadata.GetGenericMetadata(&ss.ObjectMeta, constants.K8sStatefulSet)
	km.Metadata[statefulSetCurrentVersion] = ss.Status.CurrentRevision
	km.Metadata[statefulSetUpdateVersion] = ss.Status.UpdateRevision

	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{experimentalmetricmetadata.ResourceID(ss.UID): km}
}
