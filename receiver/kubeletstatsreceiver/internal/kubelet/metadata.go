// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
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
	Labels                    map[MetadataLabel]bool
	PodsMetadata              *v1.PodList
	DetailedPVCResourceSetter func(rb *metadata.ResourceBuilder, volCacheID, volumeClaim, namespace string) error
	podLimits                 map[string]limit
	containerLimits           map[string]limit
}

type limit struct {
	cpu    float64
	memory float64
}

func newContainerLimit(resources *v1.ResourceRequirements) limit {
	if resources == nil {
		return limit{}
	}
	var containerLimit limit
	cpuLimit, err := strconv.ParseFloat(resources.Limits.Cpu().AsDec().String(), 64)
	if err == nil {
		containerLimit.cpu = cpuLimit
	}
	memoryLimit, err := strconv.ParseFloat(resources.Limits.Memory().AsDec().String(), 64)
	if err == nil {
		containerLimit.memory = memoryLimit
	}
	return containerLimit
}

func NewMetadata(labels []MetadataLabel, podsMetadata *v1.PodList,
	detailedPVCResourceSetter func(rb *metadata.ResourceBuilder, volCacheID, volumeClaim, namespace string) error) Metadata {
	m := Metadata{
		Labels:                    getLabelsMap(labels),
		PodsMetadata:              podsMetadata,
		DetailedPVCResourceSetter: detailedPVCResourceSetter,
		podLimits:                 make(map[string]limit, 0),
		containerLimits:           make(map[string]limit, 0),
	}

	if podsMetadata != nil {
		for _, pod := range podsMetadata.Items {
			var podLimit limit
			allContainersCPUDefined := true
			allContainersMemoryDefined := true
			for _, container := range pod.Status.ContainerStatuses {
				containerLimit := newContainerLimit(container.Resources)

				if allContainersCPUDefined && containerLimit.cpu == 0 {
					allContainersCPUDefined = false
					podLimit.cpu = 0
				}

				if allContainersCPUDefined {
					podLimit.cpu += containerLimit.cpu
				}

				if allContainersMemoryDefined && containerLimit.memory == 0 {
					allContainersMemoryDefined = false
					podLimit.memory = 0
				}

				if allContainersMemoryDefined {
					podLimit.memory += containerLimit.memory
				}

				m.containerLimits[string(pod.UID)+container.Name] = containerLimit
			}
			m.podLimits[string(pod.UID)] = podLimit
		}
	}

	return m
}

func getLabelsMap(metadataLabels []MetadataLabel) map[MetadataLabel]bool {
	out := make(map[MetadataLabel]bool, len(metadataLabels))
	for _, l := range metadataLabels {
		out[l] = true
	}
	return out
}

// getExtraResources gets extra resources based on provided metadata label.
func (m *Metadata) setExtraResources(rb *metadata.ResourceBuilder, podRef stats.PodReference,
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
		containerID, err := m.getContainerID(podRef.UID, extraMetadataFrom)
		if err != nil {
			return err
		}
		rb.SetContainerID(containerID)
	case MetadataLabelVolumeType:
		volume, err := m.getPodVolume(podRef.UID, extraMetadataFrom)
		if err != nil {
			return err
		}

		setResourcesFromVolume(rb, volume)

		// Get more labels from PersistentVolumeClaim volume type.
		if volume.PersistentVolumeClaim != nil {
			volCacheID := fmt.Sprintf("%s/%s", podRef.UID, extraMetadataFrom)
			err := m.DetailedPVCResourceSetter(rb, volCacheID, volume.PersistentVolumeClaim.ClaimName, podRef.Namespace)
			if err != nil {
				return fmt.Errorf("failed to set labels from volume claim: %w", err)
			}
		}
	}
	return nil
}

// getContainerID retrieves container id from metadata for given pod UID and container name,
// returns an error if no container found in the metadata that matches the requirements
// or if the apiServer returned a newly created container with empty containerID.
func (m *Metadata) getContainerID(podUID string, containerName string) (string, error) {
	uid := types.UID(podUID)
	for _, pod := range m.PodsMetadata.Items {
		if pod.UID == uid {
			for _, containerStatus := range append(pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses...) {
				if containerName == containerStatus.Name {
					if len(strings.TrimSpace(containerStatus.ContainerID)) == 0 {
						return "", fmt.Errorf("pod %q with container %q has an empty containerID", podUID, containerName)
					}
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

func (m *Metadata) getPodVolume(podUID string, volumeName string) (v1.Volume, error) {
	for _, pod := range m.PodsMetadata.Items {
		if pod.UID == types.UID(podUID) {
			for _, volume := range pod.Spec.Volumes {
				if volumeName == volume.Name {
					return volume, nil
				}
			}
		}
	}

	return v1.Volume{}, fmt.Errorf("pod %q with volume %q not found in the fetched metadata", podUID, volumeName)
}

func (m *Metadata) getPodCpuLimit(uid string) *float64 {
	podLimit, ok := m.podLimits[uid]
	if !ok {
		return nil
	}
	if podLimit.cpu > 0 {
		return &podLimit.cpu
	}
	return nil
}

func (m *Metadata) getPodMemoryLimit(uid string) *float64 {
	podLimit, ok := m.podLimits[uid]
	if !ok {
		return nil
	}
	if podLimit.memory > 0 {
		return &podLimit.memory
	}
	return nil
}

func (m *Metadata) getContainerCpuLimit(podUID string, containerName string) *float64 {
	containerLimit, ok := m.containerLimits[podUID+containerName]
	if !ok {
		return nil
	}
	if containerLimit.cpu > 0 {
		return &containerLimit.cpu
	}
	return nil
}

func (m *Metadata) getContainerMemoryLimit(podUID string, containerName string) *float64 {
	containerLimit, ok := m.containerLimits[podUID+containerName]
	if !ok {
		return nil
	}
	if containerLimit.memory > 0 {
		return &containerLimit.memory
	}
	return nil
}
