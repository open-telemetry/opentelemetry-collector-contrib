// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func getContainerResourceOptions(sPod stats.PodStats, sContainer stats.ContainerStats, k8sMetadata Metadata) ([]metadata.ResourceMetricsOption, error) {
	ro := []metadata.ResourceMetricsOption{
		metadata.WithStartTimeOverride(pcommon.NewTimestampFromTime(sContainer.StartTime.Time)),
		metadata.WithK8sPodUID(sPod.PodRef.UID),
		metadata.WithK8sPodName(sPod.PodRef.Name),
		metadata.WithK8sNamespaceName(sPod.PodRef.Namespace),
		metadata.WithK8sContainerName(sContainer.Name),
	}

	extraResources, err := k8sMetadata.getExtraResources(sPod.PodRef, MetadataLabelContainerID, sContainer.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to set extra labels from metadata: %w", err)
	}

	ro = append(ro, extraResources...)

	return ro, nil
}

func getVolumeResourceOptions(sPod stats.PodStats, vs stats.VolumeStats, k8sMetadata Metadata) ([]metadata.ResourceMetricsOption, error) {
	ro := []metadata.ResourceMetricsOption{
		metadata.WithK8sPodUID(sPod.PodRef.UID),
		metadata.WithK8sPodName(sPod.PodRef.Name),
		metadata.WithK8sNamespaceName(sPod.PodRef.Namespace),
		metadata.WithK8sVolumeName(vs.Name),
	}

	extraResources, err := k8sMetadata.getExtraResources(sPod.PodRef, MetadataLabelVolumeType, vs.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to set extra labels from metadata: %w", err)
	}

	ro = append(ro, extraResources...)

	return ro, nil
}
