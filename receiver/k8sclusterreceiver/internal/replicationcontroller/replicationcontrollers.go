// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package replicationcontroller // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replicationcontroller"

import (
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replica"
)

func GetMetrics(rc *corev1.ReplicationController) []*agentmetricspb.ExportMetricsServiceRequest {
	if rc.Spec.Replicas == nil {
		return nil
	}

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResource(rc),
			Metrics: replica.GetMetrics(
				"replication_controller",
				*rc.Spec.Replicas,
				rc.Status.AvailableReplicas,
			),
		},
	}

}

func getResource(rc *corev1.ReplicationController) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			constants.K8sKeyReplicationControllerUID:  string(rc.UID),
			constants.K8sKeyReplicationControllerName: rc.Name,
			conventions.AttributeK8SNamespaceName:     rc.Namespace,
		},
	}
}

func GetMetadata(rc *corev1.ReplicationController) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(rc.UID): metadata.GetGenericMetadata(&rc.ObjectMeta, constants.K8sKindReplicationController),
	}
}
