// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deployment // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/deployment"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

// Transform transforms the pod to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new deployment fields.
func Transform(deployment *appsv1.Deployment) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metadata.TransformObjectMeta(deployment.ObjectMeta),
		Spec: appsv1.DeploymentSpec{
			Replicas: deployment.Spec.Replicas,
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: deployment.Status.AvailableReplicas,
		},
	}
}

func GetMetrics(set receiver.CreateSettings, dep *appsv1.Deployment) pmetric.Metrics {
	mb := imetadata.NewMetricsBuilder(imetadata.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())
	mb.RecordK8sDeploymentDesiredDataPoint(ts, int64(*dep.Spec.Replicas))
	mb.RecordK8sDeploymentAvailableDataPoint(ts, int64(dep.Status.AvailableReplicas))
	rb := imetadata.NewResourceBuilder(imetadata.DefaultResourceAttributesConfig())
	rb.SetK8sDeploymentName(dep.Name)
	rb.SetK8sDeploymentUID(string(dep.UID))
	rb.SetK8sNamespaceName(dep.Namespace)
	rb.SetOpencensusResourcetype("k8s")
	return mb.Emit(imetadata.WithResource(rb.Emit()))
}

func GetMetadata(dep *appsv1.Deployment) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	rm := metadata.GetGenericMetadata(&dep.ObjectMeta, constants.K8sKindDeployment)
	rm.Metadata[conventions.AttributeK8SDeploymentName] = dep.Name
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{experimentalmetricmetadata.ResourceID(dep.UID): rm}
}
