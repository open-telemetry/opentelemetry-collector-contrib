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
	"errors"
	"fmt"
	"regexp"

	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type MetadataLabel string

// Values for MetadataLabel enum.
const (
	MetadataLabelContainerID MetadataLabel = conventions.AttributeContainerID
	MetadataLabelVolumeType  MetadataLabel = labelVolumeType
)

var supportedLabels = map[MetadataLabel]bool{
	MetadataLabelContainerID: true,
	MetadataLabelVolumeType:  true,
}

// ValidateMetadataLabelsConfig validates that provided list of metadata labels is supported
func ValidateMetadataLabelsConfig(labels []MetadataLabel) error {
	labelsFound := map[MetadataLabel]bool{}
	for _, label := range labels {
		if _, supported := supportedLabels[label]; supported {
			if _, duplicate := labelsFound[label]; duplicate {
				return fmt.Errorf("duplicate metadata label: %q", label)
			}
			labelsFound[label] = true
		} else {
			return fmt.Errorf("label %q is not supported", label)
		}
	}
	return nil
}

type Metadata struct {
	Labels                  map[MetadataLabel]bool
	PodsMetadata            *v1.PodList
	DetailedPVCLabelsSetter func(volCacheID, volumeClaim, namespace string, labels map[string]string) error
}

func NewMetadata(
	labels []MetadataLabel, podsMetadata *v1.PodList,
	detailedPVCLabelsSetter func(volCacheID, volumeClaim, namespace string, labels map[string]string) error) Metadata {
	return Metadata{
		Labels:                  getLabelsMap(labels),
		PodsMetadata:            podsMetadata,
		DetailedPVCLabelsSetter: detailedPVCLabelsSetter,
	}
}

func getLabelsMap(metadataLabels []MetadataLabel) map[MetadataLabel]bool {
	out := make(map[MetadataLabel]bool, len(metadataLabels))
	for _, l := range metadataLabels {
		out[l] = true
	}
	return out
}

// setExtraLabels sets extra labels in `labels` map based on provided metadata label.
func (m *Metadata) setExtraLabels(
	labels map[string]string, podUID string,
	extraMetadataLabel MetadataLabel, extraMetadataFrom string) error {
	// Ensure MetadataLabel exists before proceeding.
	if !m.Labels[extraMetadataLabel] || len(m.Labels) == 0 {
		return nil
	}

	// Cannot proceed, if metadata is unavailable.
	if m.PodsMetadata == nil {
		return errors.New("pods metadata were not fetched")
	}

	switch extraMetadataLabel {
	case MetadataLabelContainerID:
		containerID, err := m.getContainerID(podUID, extraMetadataFrom)
		if err != nil {
			return err
		}
		labels[conventions.AttributeContainerID] = containerID
	case MetadataLabelVolumeType:
		err := m.setExtraVolumeMetadata(podUID, extraMetadataFrom, labels)
		if err != nil {
			return err
		}
	}
	return nil
}

// getContainerID retrieves container id from metadata for given pod UID and container name,
// returns an error if no container found in the metadata that matches the requirements.
func (m *Metadata) getContainerID(podUID string, containerName string) (string, error) {
	uid := types.UID(podUID)
	for _, pod := range m.PodsMetadata.Items {
		if pod.UID == uid {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerName == containerStatus.Name {
					return stripContainerID(containerStatus.ContainerID), nil
				}
			}
		}
	}

	return "", fmt.Errorf("pod %q with container %q not found in the fetched metadata", podUID, containerName)
}

var containerSchemeRegexp = regexp.MustCompile(`^[\w_-]+://`)

// stripContainerID returns a pure container id without the runtime scheme://
func stripContainerID(id string) string {
	return containerSchemeRegexp.ReplaceAllString(id, "")
}

func (m *Metadata) setExtraVolumeMetadata(podUID string, volumeName string, labels map[string]string) error {
	uid := types.UID(podUID)
	for _, pod := range m.PodsMetadata.Items {
		if pod.UID == uid {
			for _, volume := range pod.Spec.Volumes {
				if volumeName == volume.Name {
					getLabelsFromVolume(volume, labels)
					return nil
				}
			}
		}
	}

	return fmt.Errorf("pod %q with volume %q not found in the fetched metadata", podUID, volumeName)
}
