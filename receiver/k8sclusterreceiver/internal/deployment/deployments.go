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
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/deployment/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func GetMetrics(set receiver.CreateSettings, dep *appsv1.Deployment) pmetric.Metrics {
	mb := imetadata.NewMetricsBuilder(imetadata.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())
	mb.RecordK8sDeploymentDesiredDataPoint(ts, int64(*dep.Spec.Replicas))
	mb.RecordK8sDeploymentAvailableDataPoint(ts, int64(dep.Status.AvailableReplicas))
	return mb.Emit(imetadata.WithK8sDeploymentUID(string(dep.UID)), imetadata.WithK8sDeploymentName(dep.Name), imetadata.WithK8sNamespaceName(dep.Namespace), imetadata.WithOpencensusResourcetype("k8s"))
}

func GetMetadata(dep *appsv1.Deployment) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	rm := metadata.GetGenericMetadata(&dep.ObjectMeta, constants.K8sKindDeployment)
	rm.Metadata[conventions.AttributeK8SDeploymentName] = dep.Name
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{experimentalmetricmetadata.ResourceID(dep.UID): rm}
}
