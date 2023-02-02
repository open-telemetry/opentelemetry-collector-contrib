// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hpa // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/hpa"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/hpa/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func GetMetrics(set receiver.CreateSettings, hpa *autoscalingv2beta2.HorizontalPodAutoscaler) pmetric.Metrics {
	mb := imetadata.NewMetricsBuilder(imetadata.DefaultMetricsSettings(), set)

	ts := pcommon.NewTimestampFromTime(time.Now())
	mb.RecordK8sHpaMaxReplicasDataPoint(ts, int64(hpa.Spec.MaxReplicas))
	mb.RecordK8sHpaMinReplicasDataPoint(ts, int64(*hpa.Spec.MinReplicas))
	mb.RecordK8sHpaCurrentReplicasDataPoint(ts, int64(hpa.Status.CurrentReplicas))
	mb.RecordK8sHpaDesiredReplicasDataPoint(ts, int64(hpa.Status.DesiredReplicas))
	return mb.Emit(imetadata.WithK8sHpaUID(string(hpa.UID)), imetadata.WithK8sHpaName(hpa.Name), imetadata.WithK8sNamespaceName(hpa.Namespace))
}

func GetMetadata(hpa *autoscalingv2beta2.HorizontalPodAutoscaler) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(hpa.UID): metadata.GetGenericMetadata(&hpa.ObjectMeta, "HPA"),
	}
}
