// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hpa // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/hpa"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func RecordMetricsBeta(mb *metadata.MetricsBuilder, hpa *autoscalingv2beta2.HorizontalPodAutoscaler, ts pcommon.Timestamp) {
	rb := mb.NewResourceBuilder()
	rb.SetK8sHpaUID(string(hpa.UID))
	rb.SetK8sHpaName(hpa.Name)
	rb.SetK8sNamespaceName(hpa.Namespace)
	rmb := mb.ResourceMetricsBuilder(rb.Emit())
	rmb.RecordK8sHpaMaxReplicasDataPoint(ts, int64(hpa.Spec.MaxReplicas))
	rmb.RecordK8sHpaMinReplicasDataPoint(ts, int64(*hpa.Spec.MinReplicas))
	rmb.RecordK8sHpaCurrentReplicasDataPoint(ts, int64(hpa.Status.CurrentReplicas))
	rmb.RecordK8sHpaDesiredReplicasDataPoint(ts, int64(hpa.Status.DesiredReplicas))
}

func RecordMetrics(mb *metadata.MetricsBuilder, hpa *autoscalingv2.HorizontalPodAutoscaler, ts pcommon.Timestamp) {
	rb := mb.NewResourceBuilder()
	rb.SetK8sHpaUID(string(hpa.UID))
	rb.SetK8sHpaName(hpa.Name)
	rb.SetK8sNamespaceName(hpa.Namespace)
	rmb := mb.ResourceMetricsBuilder(rb.Emit())
	rmb.RecordK8sHpaMaxReplicasDataPoint(ts, int64(hpa.Spec.MaxReplicas))
	rmb.RecordK8sHpaMinReplicasDataPoint(ts, int64(*hpa.Spec.MinReplicas))
	rmb.RecordK8sHpaCurrentReplicasDataPoint(ts, int64(hpa.Status.CurrentReplicas))
	rmb.RecordK8sHpaDesiredReplicasDataPoint(ts, int64(hpa.Status.DesiredReplicas))
}

func GetMetadata(hpa *autoscalingv2.HorizontalPodAutoscaler) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(hpa.UID): metadata.GetGenericMetadata(&hpa.ObjectMeta, "HPA"),
	}
}

func GetMetadataBeta(hpa *autoscalingv2beta2.HorizontalPodAutoscaler) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(hpa.UID): metadata.GetGenericMetadata(&hpa.ObjectMeta, "HPA"),
	}
}
