// Copyright 2020, OpenTelemetry Authors
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

package kubelet

import (
	"fmt"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

func nodeResource(s stats.NodeStats) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: "k8s", // k8s/node
		Labels: map[string]string{
			conventions.AttributeK8SNodeName: s.NodeName,
		},
	}
}

func podResource(s stats.PodStats) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: "k8s", // k8s/pod
		Labels: map[string]string{
			conventions.AttributeK8SPodUID:        s.PodRef.UID,
			conventions.AttributeK8SPodName:       s.PodRef.Name,
			conventions.AttributeK8SNamespaceName: s.PodRef.Namespace,
		},
	}
}

func containerResource(pod *resourcepb.Resource, s stats.ContainerStats, metadata Metadata) (*resourcepb.Resource, error) {
	labels := map[string]string{}
	for k, v := range pod.Labels {
		labels[k] = v
	}
	// augment the container resource with pod labels
	labels[conventions.AttributeK8SContainerName] = s.Name
	err := metadata.setExtraLabels(
		labels, labels[conventions.AttributeK8SPodUID],
		MetadataLabelContainerID, labels[conventions.AttributeK8SContainerName],
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set extra labels from metadata: %w", err)

	}
	return &resourcepb.Resource{
		Type:   "k8s", // k8s/pod/container
		Labels: labels,
	}, nil
}

func volumeResource(pod *resourcepb.Resource, vs stats.VolumeStats, metadata Metadata) (*resourcepb.Resource, error) {
	labels := map[string]string{
		labelVolumeName: vs.Name,
	}

	err := metadata.setExtraLabels(
		labels, pod.Labels[conventions.AttributeK8SPodUID],
		MetadataLabelVolumeType, labels[labelVolumeName],
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set extra labels from metadata: %w", err)
	}

	if labels[labelVolumeType] == labelValuePersistentVolumeClaim {
		volCacheID := fmt.Sprintf("%s/%s", pod.Labels[conventions.AttributeK8SPodUID], vs.Name)
		err = metadata.DetailedPVCLabelsSetter(
			volCacheID, labels[labelPersistentVolumeClaimName],
			pod.Labels[conventions.AttributeK8SNamespaceName], labels,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to set labels from volume claim: %w", err)
		}
	}

	// Collect relevant Pod labels to be able to associate the volume to it.
	labels[conventions.AttributeK8SPodUID] = pod.Labels[conventions.AttributeK8SPodUID]
	labels[conventions.AttributeK8SPodName] = pod.Labels[conventions.AttributeK8SPodName]
	labels[conventions.AttributeK8SNamespaceName] = pod.Labels[conventions.AttributeK8SNamespaceName]

	return &resourcepb.Resource{
		Type:   "k8s",
		Labels: labels,
	}, nil
}
