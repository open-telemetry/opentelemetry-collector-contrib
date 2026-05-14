// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hpa // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/hpa"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	autoscalingv2 "k8s.io/api/autoscaling/v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func RecordMetrics(mb *metadata.MetricsBuilder, hpa *autoscalingv2.HorizontalPodAutoscaler, ts pcommon.Timestamp) {
	e := metadata.NewK8sHpaEntity(string(hpa.UID))
	e.SetK8sHpaName(hpa.Name)
	e.SetK8sNamespaceName(hpa.Namespace)
	e.SetK8sHpaScaletargetrefApiversion(hpa.Spec.ScaleTargetRef.APIVersion)
	e.SetK8sHpaScaletargetrefKind(hpa.Spec.ScaleTargetRef.Kind)
	e.SetK8sHpaScaletargetrefName(hpa.Spec.ScaleTargetRef.Name)
	eb := mb.ForK8sHpa(e)
	eb.RecordK8sHpaMaxReplicasDataPoint(ts, int64(hpa.Spec.MaxReplicas))
	eb.RecordK8sHpaMinReplicasDataPoint(ts, int64(*hpa.Spec.MinReplicas))
	eb.RecordK8sHpaCurrentReplicasDataPoint(ts, int64(hpa.Status.CurrentReplicas))
	eb.RecordK8sHpaDesiredReplicasDataPoint(ts, int64(hpa.Status.DesiredReplicas))
	eb.Emit()
}

func GetMetadata(hpa *autoscalingv2.HorizontalPodAutoscaler) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(hpa.UID): metadata.GetGenericMetadata(&hpa.ObjectMeta, "HPA"),
	}
}
