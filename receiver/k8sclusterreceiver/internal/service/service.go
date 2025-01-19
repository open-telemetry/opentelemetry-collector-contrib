// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package service // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/service"
import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

// Transform transforms the pod to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new service fields.
func Transform(service *corev1.Service) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metadata.TransformObjectMeta(service.ObjectMeta),
		Spec: corev1.ServiceSpec{
			Selector: service.Spec.Selector,
		},
	}
}

// GetPodServiceTags returns a set of services associated with the pod.
func GetPodServiceTags(pod *corev1.Pod, services cache.Store) map[string]string {
	properties := map[string]string{}

	for _, ser := range services.List() {
		serObj := ser.(*corev1.Service)
		if serObj.Namespace == pod.Namespace &&
			labels.Set(serObj.Spec.Selector).AsSelectorPreValidated().Matches(labels.Set(pod.Labels)) {
			properties[fmt.Sprintf("%s%s", constants.K8sServicePrefix, serObj.Name)] = ""
		}
	}

	return properties
}
