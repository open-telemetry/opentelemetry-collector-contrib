// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package replicationcontroller // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replicationcontroller"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func RecordMetrics(mb *metadata.MetricsBuilder, rc *corev1.ReplicationController, ts pcommon.Timestamp) {
	e := metadata.NewK8sReplicationcontrollerEntity(string(rc.UID))
	e.SetK8sReplicationcontrollerName(rc.Name)
	e.SetK8sNamespaceName(rc.Namespace)
	eb := mb.ForK8sReplicationcontroller(e)

	if rc.Spec.Replicas != nil {
		eb.RecordK8sReplicationControllerDesiredDataPoint(ts, int64(*rc.Spec.Replicas))
		eb.RecordK8sReplicationControllerAvailableDataPoint(ts, int64(rc.Status.AvailableReplicas))
	}

	eb.Emit()
}

func GetMetadata(rc *corev1.ReplicationController) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(rc.UID): metadata.GetGenericMetadata(&rc.ObjectMeta, constants.K8sKindReplicationController),
	}
}
