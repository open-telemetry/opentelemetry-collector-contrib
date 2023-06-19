// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

var commonPodMetadata = map[string]string{
	"foo":                    "bar",
	"foo1":                   "",
	"pod.creation_timestamp": "0001-01-01T00:00:00Z",
}

var allPodMetadata = func(metadata map[string]string) map[string]string {
	out := maps.MergeStringMaps(metadata, commonPodMetadata)
	return out
}

func TestDataCollectorSyncMetadata(t *testing.T) {
	tests := []struct {
		name          string
		metadataStore *metadata.Store
		resource      interface{}
		want          map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata
	}{
		{
			name:          "Pod and container metadata simple case",
			metadataStore: &metadata.Store{},
			resource: testutils.NewPodWithContainer(
				"0",
				testutils.NewPodSpecWithContainer("container-name"),
				testutils.NewPodStatusWithContainer("container-name", "container-id"),
			),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-pod-0-uid"): {
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "test-pod-0-uid",
					Metadata:      commonPodMetadata,
				},
				experimentalmetricmetadata.ResourceID("container-id"): {
					ResourceIDKey: "container.id",
					ResourceID:    "container-id",
					Metadata: map[string]string{
						"container.status": "running",
					},
				},
			},
		},
		{
			name:          "Pod with Owner Reference",
			metadataStore: &metadata.Store{},
			resource: testutils.WithOwnerReferences([]v1.OwnerReference{
				{
					Kind: "StatefulSet",
					Name: "test-statefulset-0",
					UID:  "test-statefulset-0-uid",
				},
			}, testutils.NewPodWithContainer("0", &corev1.PodSpec{}, &corev1.PodStatus{})),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-pod-0-uid"): {
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "test-pod-0-uid",
					Metadata: allPodMetadata(map[string]string{
						"k8s.workload.kind":    "StatefulSet",
						"k8s.workload.name":    "test-statefulset-0",
						"k8s.statefulset.name": "test-statefulset-0",
						"k8s.statefulset.uid":  "test-statefulset-0-uid",
					}),
				},
			},
		},
		{
			name: "Pod with Service metadata",
			metadataStore: &metadata.Store{
				Services: &testutils.MockStore{
					Cache: map[string]interface{}{
						"test-namespace/test-service": &corev1.Service{
							ObjectMeta: v1.ObjectMeta{
								Name:      "test-service",
								Namespace: "test-namespace",
								UID:       "test-service-uid",
							},
							Spec: corev1.ServiceSpec{
								Selector: map[string]string{
									"k8s-app": "my-app",
								},
							},
						},
					},
				},
			},
			resource: podWithAdditionalLabels(
				map[string]string{"k8s-app": "my-app"},
				testutils.NewPodWithContainer("0", &corev1.PodSpec{}, &corev1.PodStatus{}),
			),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-pod-0-uid"): {
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "test-pod-0-uid",
					Metadata: allPodMetadata(map[string]string{
						"k8s.service.test-service": "",
						"k8s-app":                  "my-app",
					}),
				},
			},
		},
		{
			name:          "Daemonset simple case",
			metadataStore: &metadata.Store{},
			resource:      testutils.NewDaemonset("1"),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-daemonset-1-uid"): {
					ResourceIDKey: "k8s.daemonset.uid",
					ResourceID:    "test-daemonset-1-uid",
					Metadata: map[string]string{
						"k8s.workload.kind":            "DaemonSet",
						"k8s.workload.name":            "test-daemonset-1",
						"daemonset.creation_timestamp": "0001-01-01T00:00:00Z",
					},
				},
			},
		},
		{
			name:          "Deployment simple case",
			metadataStore: &metadata.Store{},
			resource:      testutils.NewDeployment("1"),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-deployment-1-uid"): {
					ResourceIDKey: "k8s.deployment.uid",
					ResourceID:    "test-deployment-1-uid",
					Metadata: map[string]string{
						"k8s.workload.kind":             "Deployment",
						"k8s.workload.name":             "test-deployment-1",
						"k8s.deployment.name":           "test-deployment-1",
						"deployment.creation_timestamp": "0001-01-01T00:00:00Z",
					},
				},
			},
		},
		{
			name:          "HPA simple case",
			metadataStore: &metadata.Store{},
			resource:      testutils.NewHPA("1"),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-hpa-1-uid"): {
					ResourceIDKey: "k8s.hpa.uid",
					ResourceID:    "test-hpa-1-uid",
					Metadata: map[string]string{
						"k8s.workload.kind":      "HPA",
						"k8s.workload.name":      "test-hpa-1",
						"hpa.creation_timestamp": "0001-01-01T00:00:00Z",
					},
				},
			},
		},
		{
			name:          "Job simple case",
			metadataStore: &metadata.Store{},
			resource:      testutils.NewJob("1"),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-job-1-uid"): {
					ResourceIDKey: "k8s.job.uid",
					ResourceID:    "test-job-1-uid",
					Metadata: map[string]string{
						"foo":                    "bar",
						"foo1":                   "",
						"k8s.workload.kind":      "Job",
						"k8s.workload.name":      "test-job-1",
						"job.creation_timestamp": "0001-01-01T00:00:00Z",
					},
				},
			},
		},
		{
			name:          "Node simple case",
			metadataStore: &metadata.Store{},
			resource:      testutils.NewNode("1"),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-node-1-uid"): {
					ResourceIDKey: "k8s.node.uid",
					ResourceID:    "test-node-1-uid",
					Metadata: map[string]string{
						"foo":                     "bar",
						"foo1":                    "",
						"k8s.node.name":           "test-node-1",
						"node.creation_timestamp": "0001-01-01T00:00:00Z",
					},
				},
			},
		},
		{
			name:          "ReplicaSet simple case",
			metadataStore: &metadata.Store{},
			resource:      testutils.NewReplicaSet("1"),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-replicaset-1-uid"): {
					ResourceIDKey: "k8s.replicaset.uid",
					ResourceID:    "test-replicaset-1-uid",
					Metadata: map[string]string{
						"foo":                           "bar",
						"foo1":                          "",
						"k8s.workload.kind":             "ReplicaSet",
						"k8s.workload.name":             "test-replicaset-1",
						"replicaset.creation_timestamp": "0001-01-01T00:00:00Z",
					},
				},
			},
		},
		{
			name:          "ReplicationController simple case",
			metadataStore: &metadata.Store{},
			resource: &corev1.ReplicationController{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-replicationcontroller-1",
					Namespace: "test-namespace",
					UID:       types.UID("test-replicationcontroller-1-uid"),
				},
			},
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-replicationcontroller-1-uid"): {
					ResourceIDKey: "k8s.replicationcontroller.uid",
					ResourceID:    "test-replicationcontroller-1-uid",
					Metadata: map[string]string{
						"k8s.workload.kind":                        "ReplicationController",
						"k8s.workload.name":                        "test-replicationcontroller-1",
						"replicationcontroller.creation_timestamp": "0001-01-01T00:00:00Z",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		observedLogger, _ := observer.New(zapcore.WarnLevel)
		set := receivertest.NewNopCreateSettings()
		set.TelemetrySettings.Logger = zap.New(observedLogger)
		t.Run(tt.name, func(t *testing.T) {
			dc := &DataCollector{
				settings:               set,
				metadataStore:          tt.metadataStore,
				nodeConditionsToReport: []string{},
			}

			actual := dc.SyncMetadata(tt.resource)
			require.Equal(t, len(tt.want), len(actual))

			for key, item := range tt.want {
				got, exists := actual[key]
				require.True(t, exists)
				require.Equal(t, *item, *got)
			}
		})
	}
}

func podWithAdditionalLabels(labels map[string]string, pod *corev1.Pod) interface{} {
	if pod.Labels == nil {
		pod.Labels = make(map[string]string, len(labels))
	}

	for k, v := range labels {
		pod.Labels[k] = v
	}

	return pod
}
