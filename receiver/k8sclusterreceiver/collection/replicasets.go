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

package collection

import (
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opentelemetry.io/collector/translator/conventions"
	appsv1 "k8s.io/api/apps/v1"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

func getMetricsForReplicaSet(rs *appsv1.ReplicaSet) []*resourceMetrics {
	if rs.Spec.Replicas == nil {
		return nil
	}

	return []*resourceMetrics{
		{
			resource: getResourceForReplicaSet(rs),
			metrics: getReplicaMetrics(
				"replicaset",
				*rs.Spec.Replicas,
				rs.Status.AvailableReplicas,
			),
		},
	}

}

func getResourceForReplicaSet(rs *appsv1.ReplicaSet) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: k8sType,
		Labels: map[string]string{
			conventions.AttributeK8sReplicaSetUID: string(rs.UID),
			conventions.AttributeK8sReplicaSet:    rs.Name,
			conventions.AttributeK8sNamespace:     rs.Namespace,
			conventions.AttributeK8sCluster:       rs.ClusterName,
		},
	}
}

func getMetadataForReplicaSet(rs *appsv1.ReplicaSet) map[metadata.ResourceID]*KubernetesMetadata {
	return map[metadata.ResourceID]*KubernetesMetadata{
		metadata.ResourceID(rs.UID): getGenericMetadata(&rs.ObjectMeta, k8sKindReplicaSet),
	}
}
