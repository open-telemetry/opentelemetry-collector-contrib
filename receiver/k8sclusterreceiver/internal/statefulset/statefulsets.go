// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statefulset // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/statefulset"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// Keys for stateful set metadata.
	statefulSetCurrentVersion = "current_revision"
	statefulSetUpdateVersion  = "update_revision"
)

// Transform transforms the pod to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new statefulset fields.
func Transform(statefulset *appsv1.StatefulSet) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metadata.TransformObjectMeta(statefulset.ObjectMeta),
		Spec: appsv1.StatefulSetSpec{
			Replicas: statefulset.Spec.Replicas,
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas:   statefulset.Status.ReadyReplicas,
			CurrentReplicas: statefulset.Status.CurrentReplicas,
			UpdatedReplicas: statefulset.Status.UpdatedReplicas,
		},
	}
}

func RecordMetrics(mb *imetadata.MetricsBuilder, ss *appsv1.StatefulSet, ts pcommon.Timestamp) {
	if ss.Spec.Replicas == nil {
		return
	}
	mb.RecordK8sStatefulsetDesiredPodsDataPoint(ts, int64(*ss.Spec.Replicas))
	mb.RecordK8sStatefulsetReadyPodsDataPoint(ts, int64(ss.Status.ReadyReplicas))
	mb.RecordK8sStatefulsetCurrentPodsDataPoint(ts, int64(ss.Status.CurrentReplicas))
	mb.RecordK8sStatefulsetUpdatedPodsDataPoint(ts, int64(ss.Status.UpdatedReplicas))
	rb := mb.NewResourceBuilder()
	rb.SetK8sStatefulsetUID(string(ss.UID))
	rb.SetK8sStatefulsetName(ss.Name)
	rb.SetK8sNamespaceName(ss.Namespace)
	mb.EmitForResource(imetadata.WithResource(rb.Emit()))
}

func GetMetadata(ss *appsv1.StatefulSet) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	km := metadata.GetGenericMetadata(&ss.ObjectMeta, constants.K8sStatefulSet)
	km.Metadata[statefulSetCurrentVersion] = ss.Status.CurrentRevision
	km.Metadata[statefulSetUpdateVersion] = ss.Status.UpdateRevision

	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{experimentalmetricmetadata.ResourceID(ss.UID): km}
}
