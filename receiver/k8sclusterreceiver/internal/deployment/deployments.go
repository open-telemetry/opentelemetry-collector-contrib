// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deployment // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/deployment"

import (
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replica"
)

func GetMetrics(dep *appsv1.Deployment) []*agentmetricspb.ExportMetricsServiceRequest {
	if dep.Spec.Replicas == nil {
		return nil
	}

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResource(dep),
			Metrics: replica.GetMetrics(
				"deployment",
				*dep.Spec.Replicas,
				dep.Status.AvailableReplicas,
			),
		},
	}
}

func getResource(dep *appsv1.Deployment) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			conventions.AttributeK8SDeploymentUID:  string(dep.UID),
			conventions.AttributeK8SDeploymentName: dep.Name,
			conventions.AttributeK8SNamespaceName:  dep.Namespace,
		},
	}
}

func GetMetadata(dep *appsv1.Deployment) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	rm := metadata.GetGenericMetadata(&dep.ObjectMeta, constants.K8sKindDeployment)
	rm.Metadata[conventions.AttributeK8SDeploymentName] = dep.Name
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{experimentalmetricmetadata.ResourceID(dep.UID): rm}
}
