// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
