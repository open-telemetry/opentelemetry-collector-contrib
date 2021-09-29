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
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	appsv1 "k8s.io/api/apps/v1"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

func getMetricsForDeployment(dep *appsv1.Deployment) []*resourceMetrics {
	if dep.Spec.Replicas == nil {
		return nil
	}

	return []*resourceMetrics{
		{
			resource: getResourceForDeployment(dep),
			metrics: getReplicaMetrics(
				"deployment",
				*dep.Spec.Replicas,
				dep.Status.AvailableReplicas,
			),
		},
	}
}

func getResourceForDeployment(dep *appsv1.Deployment) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: k8sType,
		Labels: map[string]string{
			conventions.AttributeK8SDeploymentUID:  string(dep.UID),
			conventions.AttributeK8SDeploymentName: dep.Name,
			conventions.AttributeK8SNamespaceName:  dep.Namespace,
			conventions.AttributeK8SClusterName:    dep.ClusterName,
		},
	}
}

func getMetadataForDeployment(dep *appsv1.Deployment) map[metadata.ResourceID]*KubernetesMetadata {
	rm := getGenericMetadata(&dep.ObjectMeta, k8sKindDeployment)
	rm.metadata[conventions.AttributeK8SDeploymentName] = dep.Name
	return map[metadata.ResourceID]*KubernetesMetadata{metadata.ResourceID(dep.UID): rm}
}
