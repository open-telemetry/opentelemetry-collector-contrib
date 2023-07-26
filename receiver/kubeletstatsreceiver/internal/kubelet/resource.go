// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func getContainerResource(rb *metadata.ResourceBuilder, sPod stats.PodStats, sContainer stats.ContainerStats,
	k8sMetadata Metadata) (pcommon.Resource, error) {
	rb.SetK8sPodUID(sPod.PodRef.UID)
	rb.SetK8sPodName(sPod.PodRef.Name)
	rb.SetK8sNamespaceName(sPod.PodRef.Namespace)
	rb.SetK8sContainerName(sContainer.Name)

	err := k8sMetadata.setExtraResources(rb, sPod.PodRef, MetadataLabelContainerID, sContainer.Name)
	if err != nil {
		return rb.Emit(), fmt.Errorf("failed to set extra labels from metadata: %w", err)
	}

	return rb.Emit(), nil
}

func getVolumeResourceOptions(rb *metadata.ResourceBuilder, sPod stats.PodStats, vs stats.VolumeStats,
	k8sMetadata Metadata) (pcommon.Resource, error) {
	rb.SetK8sPodUID(sPod.PodRef.UID)
	rb.SetK8sPodName(sPod.PodRef.Name)
	rb.SetK8sNamespaceName(sPod.PodRef.Namespace)
	rb.SetK8sVolumeName(vs.Name)

	err := k8sMetadata.setExtraResources(rb, sPod.PodRef, MetadataLabelVolumeType, vs.Name)
	if err != nil {
		return rb.Emit(), fmt.Errorf("failed to set extra labels from metadata: %w", err)
	}

	return rb.Emit(), nil
}
