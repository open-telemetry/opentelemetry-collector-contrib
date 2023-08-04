// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	v_one "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
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
	NodesMetadata             *v1.NodeList
	DetailedPVCResourceSetter func(rb *metadata.ResourceBuilder, volCacheID, volumeClaim, namespace string) ([]metadata.ResourceMetricsOption, error)
}

func NewMetadata(labels []MetadataLabel, podsMetadata *v1.PodList, nodesMetadata *v1.NodeList,
	detailedPVCResourceSetter func(rb *metadata.ResourceBuilder, volCacheID, volumeClaim, namespace string) ([]metadata.ResourceMetricsOption, error)) Metadata {
	return Metadata{
		Labels:                    getLabelsMap(labels),
		PodsMetadata:              podsMetadata,
		NodesMetadata:             nodesMetadata,
		DetailedPVCResourceSetter: detailedPVCResourceSetter,
	}
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

	// Cannot proceed, if metadata is unavailable.
	if m.NodesMetadata == nil {
		return nil, errors.New("nodes metadata were not fetched")
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

// getNodeUID retrieves k8s.node.uid from metadata for given pod UID and container name,
// returns an error if no container found in the metadata that matches the requirements.
func (m *Metadata) getNodeUID(nodeName string) (string, error) {
	if m.NodesMetadata != nil {
		for _, node := range m.NodesMetadata.Items {
			if node.ObjectMeta.Name == nodeName {
				return string(node.ObjectMeta.UID), nil
			}
		}
	}
	return "", nil
}

// getServiceName retrieves k8s.service.name from metadata for given pod uid,
// returns an error if no service found in the metadata that matches the requirements.
func (m *Metadata) getServiceName(podUID string, client k8s.Interface) (string, error) {
	if m.PodsMetadata == nil {
		return "", errors.New("pods metadata were not fetched")
	}

	uid := types.UID(podUID)
	var service *corev1.Service
	for _, pod := range m.PodsMetadata.Items {
		if pod.UID == uid {
			serviceList, err := client.CoreV1().Services(pod.Namespace).List(context.TODO(), v_one.ListOptions{})
			if err != nil {
				return "", fmt.Errorf("failed to fetch service list for POD: %w", err)
			}
			for _, svc := range serviceList.Items {
				if svc.Spec.Selector != nil {
					if labels.Set(svc.Spec.Selector).AsSelectorPreValidated().Matches(labels.Set(pod.Labels)) {
						service = &svc
						return service.Name, nil
					}
				}
			}
		}
	}

	return "", nil
}

// getServiceAccountName retrieves k8s.service_account.name from metadata for given pod uid,
// returns an error if no service account found in the metadata that matches the requirements.
func (m *Metadata) getServiceAccountName(podUID string) (string, error) {
	if m.PodsMetadata == nil {
		return "", errors.New("pods metadata were not fetched")
	}

	uid := types.UID(podUID)
	for _, pod := range m.PodsMetadata.Items {
		if pod.UID == uid {
			return pod.Spec.ServiceAccountName, nil
		}
	}
	return "", nil
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
