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
	"regexp"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type MetadataLabel string

const (
	MetadataLabelContainerID MetadataLabel = labelContainerID
)

var supportedLabels = map[MetadataLabel]bool{
	MetadataLabelContainerID: true,
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
	Labels       []MetadataLabel
	PodsMetadata *v1.PodList
}

func NewMetadata(labels []MetadataLabel, podsMetadata *v1.PodList) Metadata {
	return Metadata{
		Labels:       labels,
		PodsMetadata: podsMetadata,
	}
}

// setExtraLabels sets extra labels in `lables` map based on available metadata
func (m *Metadata) setExtraLabels(labels map[string]string, podUID string, containerName string) error {
	for _, label := range m.Labels {
		switch label {
		case MetadataLabelContainerID:
			containerID, err := m.getContainerID(podUID, containerName)
			if err != nil {
				return err
			}
			labels[labelContainerID] = containerID
			return nil
		}
	}
	return nil
}

// getContainerID retrieves container id from metadata for given pod UID and container name,
// returns an error if no container found in the metadata that matches the requirements.
func (m *Metadata) getContainerID(podUID string, containerName string) (string, error) {
	if m.PodsMetadata == nil {
		return "", errors.New("pods metadata were not fetched")
	}

	for _, pod := range m.PodsMetadata.Items {
		if pod.UID == types.UID(podUID) {
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
