// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statefulset // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/statefulset"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/statefulset/internal/metadata"
)

const (
	// Keys for stateful set metadata.
	statefulSetCurrentVersion = "current_revision"
	statefulSetUpdateVersion  = "update_revision"
)

func GetMetrics(set receiver.CreateSettings, ss *appsv1.StatefulSet) pmetric.Metrics {
	if ss.Spec.Replicas == nil {
		return pmetric.NewMetrics()
	}
	mb := imetadata.NewMetricsBuilder(imetadata.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())
	mb.RecordK8sStatefulsetDesiredPodsDataPoint(ts, int64(*ss.Spec.Replicas))
	mb.RecordK8sStatefulsetReadyPodsDataPoint(ts, int64(ss.Status.ReadyReplicas))
	mb.RecordK8sStatefulsetCurrentPodsDataPoint(ts, int64(ss.Status.CurrentReplicas))
	mb.RecordK8sStatefulsetUpdatedPodsDataPoint(ts, int64(ss.Status.UpdatedReplicas))
	return mb.Emit(imetadata.WithK8sStatefulsetUID(string(ss.UID)), imetadata.WithK8sStatefulsetName(ss.Name), imetadata.WithK8sNamespaceName(ss.Namespace), imetadata.WithOpencensusResourcetype("k8s"))
}

func GetMetadata(ss *appsv1.StatefulSet) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	km := metadata.GetGenericMetadata(&ss.ObjectMeta, constants.K8sStatefulSet)
	km.Metadata[statefulSetCurrentVersion] = ss.Status.CurrentRevision
	km.Metadata[statefulSetUpdateVersion] = ss.Status.UpdateRevision

	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{experimentalmetricmetadata.ResourceID(ss.UID): km}
}
