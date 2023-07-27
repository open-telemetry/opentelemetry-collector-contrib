// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package replicationcontroller // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replicationcontroller"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadataphase "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replicationcontroller/internal/metadata"
)

func GetMetrics(set receiver.CreateSettings, rc *corev1.ReplicationController) pmetric.Metrics {
	mbphase := imetadataphase.NewMetricsBuilder(imetadataphase.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())

	if rc.Spec.Replicas != nil {
		mbphase.RecordK8sReplicationControllerDesiredDataPoint(ts, int64(*rc.Spec.Replicas))
		mbphase.RecordK8sReplicationControllerAvailableDataPoint(ts, int64(rc.Status.AvailableReplicas))
	}

	rb := imetadataphase.NewResourceBuilder(imetadataphase.DefaultResourceAttributesConfig())
	rb.SetK8sNamespaceName(rc.Namespace)
	rb.SetK8sReplicationcontrollerName(rc.Name)
	rb.SetK8sReplicationcontrollerUID(string(rc.UID))
	rb.SetOpencensusResourcetype("k8s")
	return mbphase.Emit(imetadataphase.WithResource(rb.Emit()))
}

func GetMetadata(rc *corev1.ReplicationController) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(rc.UID): metadata.GetGenericMetadata(&rc.ObjectMeta, constants.K8sKindReplicationController),
	}
}
