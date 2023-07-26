// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package demonset // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/demonset"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	imetadataphase "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/demonset/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

// Transform transforms the pod to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new daemonset fields.
func Transform(ds *appsv1.DaemonSet) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metadata.TransformObjectMeta(ds.ObjectMeta),
		Status: appsv1.DaemonSetStatus{
			CurrentNumberScheduled: ds.Status.CurrentNumberScheduled,
			DesiredNumberScheduled: ds.Status.DesiredNumberScheduled,
			NumberMisscheduled:     ds.Status.NumberMisscheduled,
			NumberReady:            ds.Status.NumberReady,
		},
	}
}

func GetMetrics(set receiver.CreateSettings, ds *appsv1.DaemonSet) pmetric.Metrics {
	mbphase := imetadataphase.NewMetricsBuilder(imetadataphase.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())
	mbphase.RecordK8sDaemonsetCurrentScheduledNodesDataPoint(ts, int64(ds.Status.CurrentNumberScheduled))
	mbphase.RecordK8sDaemonsetDesiredScheduledNodesDataPoint(ts, int64(ds.Status.DesiredNumberScheduled))
	mbphase.RecordK8sDaemonsetMisscheduledNodesDataPoint(ts, int64(ds.Status.NumberMisscheduled))
	mbphase.RecordK8sDaemonsetReadyNodesDataPoint(ts, int64(ds.Status.NumberReady))

	rb := imetadataphase.NewResourceBuilder(imetadataphase.DefaultResourceAttributesConfig())
	rb.SetK8sNamespaceName(ds.Namespace)
	rb.SetK8sDaemonsetName(ds.Name)
	rb.SetK8sDaemonsetUID(string(ds.UID))
	rb.SetOpencensusResourcetype("k8s")
	return mbphase.Emit(imetadataphase.WithResource(rb.Emit()))
}

func GetMetadata(ds *appsv1.DaemonSet) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(ds.UID): metadata.GetGenericMetadata(&ds.ObjectMeta, constants.K8sKindDaemonSet),
	}
}
