// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package replicaset // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replicaset"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadataphase "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
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

func GetMetrics(set receiver.CreateSettings, rs *appsv1.ReplicaSet) pmetric.Metrics {

	mbphase := imetadataphase.NewMetricsBuilder(imetadataphase.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())
	if rs.Spec.Replicas != nil {
		mbphase.RecordK8sReplicasetDesiredDataPoint(ts, int64(*rs.Spec.Replicas))
		mbphase.RecordK8sReplicasetAvailableDataPoint(ts, int64(rs.Status.AvailableReplicas))
	}

	rb := imetadataphase.NewResourceBuilder(imetadataphase.DefaultResourceAttributesConfig())
	rb.SetK8sNamespaceName(rs.Namespace)
	rb.SetK8sReplicasetName(rs.Name)
	rb.SetK8sReplicasetUID(string(rs.UID))
	rb.SetOpencensusResourcetype("k8s")
	return mbphase.Emit(imetadataphase.WithResource(rb.Emit()))
}

func GetMetadata(rs *appsv1.ReplicaSet) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(rs.UID): metadata.GetGenericMetadata(&rs.ObjectMeta, constants.K8sKindReplicaSet),
	}
}
