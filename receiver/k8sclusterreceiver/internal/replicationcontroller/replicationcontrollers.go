// Copyright The OpenTelemetry Authors
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
