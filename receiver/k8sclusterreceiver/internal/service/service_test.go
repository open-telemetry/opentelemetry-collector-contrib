// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
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

func TestGetPodServiceTags(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
	}
	tests := []struct {
		name     string
		services cache.Store
		want     map[string]string
	}{
		{
			name: "no services",
			services: &testutils.MockStore{
				Cache: map[string]any{},
			},
			want: map[string]string{},
		},
		{
			name: "no matching services",
			services: &testutils.MockStore{
				Cache: map[string]any{
					"my-service": &corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "my-service",
							Namespace: "default",
						},
						Spec: corev1.ServiceSpec{
							Selector: map[string]string{
								"app": "another-app",
							},
						},
					},
				},
			},
			want: map[string]string{},
		},
		{
			name:     "one service",
			services: generateFakeServices(1),
			want: map[string]string{
				"k8s.service.service-0": "",
			},
		},
		{
			name:     "ten services",
			services: generateFakeServices(10),
			want: map[string]string{
				"k8s.service.service-0": "",
				"k8s.service.service-1": "",
				"k8s.service.service-2": "",
				"k8s.service.service-3": "",
				"k8s.service.service-4": "",
				"k8s.service.service-5": "",
				"k8s.service.service-6": "",
				"k8s.service.service-7": "",
				"k8s.service.service-8": "",
				"k8s.service.service-9": "",
			},
		},
		{
			name:     "more than ten services",
			services: generateFakeServices(11),
			want:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPodServiceTags(pod, tt.services)
			assert.Equal(t, tt.want, got)
		})
	}
}

func generateFakeServices(count int) cache.Store {
	c := make(map[string]any)
	for i := 0; i < count; i++ {
		c[fmt.Sprintf("service-%d", i)] = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("service-%d", i),
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "my-app",
				},
			},
		}
	}
	return &testutils.MockStore{Cache: c}
}
