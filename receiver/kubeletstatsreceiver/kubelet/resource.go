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
	"github.com/pkg/errors"
	"go.opentelemetry.io/collector/translator/conventions"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

func nodeResource(s stats.NodeStats) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: "k8s", // k8s/node
		Labels: map[string]string{
			labelNodeName: s.NodeName,
		},
	}
}

func podResource(s stats.PodStats) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: "k8s", // k8s/pod
		Labels: map[string]string{
			conventions.AttributeK8sPodUID:    s.PodRef.UID,
			conventions.AttributeK8sPod:       s.PodRef.Name,
			conventions.AttributeK8sNamespace: s.PodRef.Namespace,
		},
	}
}

func containerResource(pod *resourcepb.Resource, s stats.ContainerStats, metadata Metadata) (*resourcepb.Resource, error) {
	labels := map[string]string{}
	for k, v := range pod.Labels {
		labels[k] = v
	}
	// augment the container resource with pod labels
	labels[conventions.AttributeK8sContainer] = s.Name
	err := metadata.setExtraLabels(
		labels, labels[conventions.AttributeK8sPodUID],
		MetadataLabelContainerID, labels[conventions.AttributeK8sContainer],
	)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to set extra labels from metadata")

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
		labels, pod.Labels[conventions.AttributeK8sPodUID],
		MetadataLabelVolumeType, labels[labelVolumeName],
	)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to set extra labels from metadata")
	}

	if labels[labelVolumeType] == labelValuePersistentVolumeClaim {
		volCacheID := fmt.Sprintf("%s/%s", pod.Labels[conventions.AttributeK8sPodUID], vs.Name)
		err = metadata.DetailedPVCLabelsSetter(
			volCacheID, labels[labelPersistentVolumeClaimName],
			pod.Labels[conventions.AttributeK8sNamespace], labels,
		)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to set labels from volume claim")
		}
	}

	// Collect relevant Pod labels to be able to associate the volume to it.
	labels[conventions.AttributeK8sPodUID] = pod.Labels[conventions.AttributeK8sPodUID]
	labels[conventions.AttributeK8sPod] = pod.Labels[conventions.AttributeK8sPod]
	labels[conventions.AttributeK8sNamespace] = pod.Labels[conventions.AttributeK8sNamespace]

	return &resourcepb.Resource{
		Type:   "k8s",
		Labels: labels,
	}, nil
}
