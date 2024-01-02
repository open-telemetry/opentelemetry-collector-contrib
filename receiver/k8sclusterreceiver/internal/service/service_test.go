// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTransform(t *testing.T) {
	originalService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
			Annotations: map[string]string{
				"annotation1": "value1",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "my-app",
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	wantService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "my-app",
			},
		},
	}
	assert.EqualValues(t, wantService, Transform(originalService))
}
