// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package replicaset // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replicaset"

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

// Transform transforms the replica set to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new replicaset fields.
func Transform(rs *appsv1.ReplicaSet) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metadata.TransformObjectMeta(rs.ObjectMeta),
		Spec: appsv1.ReplicaSetSpec{
			Replicas: rs.Spec.Replicas,
		},
		Status: appsv1.ReplicaSetStatus{
			AvailableReplicas: rs.Status.AvailableReplicas,
		},
	}
}

func GetMetrics(rs *appsv1.ReplicaSet) []*agentmetricspb.ExportMetricsServiceRequest {
	if rs.Spec.Replicas == nil {
		return nil
	}

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResource(rs),
			Metrics: replica.GetMetrics(
				"replicaset",
				*rs.Spec.Replicas,
				rs.Status.AvailableReplicas,
			),
		},
	}

}

func getResource(rs *appsv1.ReplicaSet) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			conventions.AttributeK8SReplicaSetUID:  string(rs.UID),
			conventions.AttributeK8SReplicaSetName: rs.Name,
			conventions.AttributeK8SNamespaceName:  rs.Namespace,
		},
	}
}

func GetMetadata(rs *appsv1.ReplicaSet) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(rs.UID): metadata.GetGenericMetadata(&rs.ObjectMeta, constants.K8sKindReplicaSet),
	}
}
