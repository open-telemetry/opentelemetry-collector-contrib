// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

var commonPodMetadata = map[string]string{
	"foo":                    "bar",
	"foo1":                   "",
	"pod.creation_timestamp": "0001-01-01T00:00:00Z",
}

func TestSetupMetadataExporters(t *testing.T) {
	type fields struct {
		metadataConsumers []metadataConsumer
	}
	type args struct {
		exporters                   map[component.ID]component.Component
		metadataExportersFromConfig []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Unsupported exporter",
			fields{},
			args{
				exporters: map[component.ID]component.Component{
					component.MustNewID("nop"): MockExporter{},
				},
				metadataExportersFromConfig: []string{"nop"},
			},
			true,
		},
		{
			"Supported exporter",
			fields{
				metadataConsumers: []metadataConsumer{(&mockExporterWithK8sMetadata{}).ConsumeMetadata},
			},
			args{
				exporters: map[component.ID]component.Component{
					component.MustNewID("nop"): mockExporterWithK8sMetadata{},
				},
				metadataExportersFromConfig: []string{"nop"},
			},
			false,
		},
		{
			"Nonexistent exporter",
			fields{
				metadataConsumers: []metadataConsumer{},
			},
			args{
				exporters: map[component.ID]component.Component{
					component.MustNewID("nop"): mockExporterWithK8sMetadata{},
				},
				metadataExportersFromConfig: []string{"nop/1"},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := &resourceWatcher{
				logger: zap.NewNop(),
			}
			if err := rw.setupMetadataExporters(tt.args.exporters, tt.args.metadataExportersFromConfig); (err != nil) != tt.wantErr {
				t.Errorf("setupMetadataExporters() error = %v, wantErr %v", err, tt.wantErr)
			}

			require.Equal(t, len(tt.fields.metadataConsumers), len(rw.metadataConsumers))
		})
	}
}

func TestIsKindSupported(t *testing.T) {
	tests := []struct {
		name     string
		client   *fake.Clientset
		gvk      schema.GroupVersionKind
		expected bool
	}{
		{
			name:     "nothing_supported",
			client:   fake.NewClientset(),
			gvk:      gvk.Pod,
			expected: false,
		},
		{
			name:     "all_kinds_supported",
			client:   newFakeClientWithAllResources(),
			gvk:      gvk.Pod,
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := &resourceWatcher{
				client: tt.client,
				logger: zap.NewNop(),
			}
			supported, err := rw.isKindSupported(tt.gvk)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, supported)
		})
	}
}

func TestPrepareSharedInformerFactory(t *testing.T) {
	tests := []struct {
		name   string
		client *fake.Clientset
	}{
		{
			name:   "new_server_version",
			client: newFakeClientWithAllResources(),
		},
		{
			name: "old_server_version", // With no batch/v1.CronJob support.
			client: func() *fake.Clientset {
				client := fake.NewClientset()
				client.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: "v1",
						APIResources: []metav1.APIResource{
							gvkToAPIResource(gvk.Pod),
							gvkToAPIResource(gvk.Node),
							gvkToAPIResource(gvk.Namespace),
							gvkToAPIResource(gvk.ReplicationController),
							gvkToAPIResource(gvk.ResourceQuota),
							gvkToAPIResource(gvk.Service),
						},
					},
					{
						GroupVersion: "apps/v1",
						APIResources: []metav1.APIResource{
							gvkToAPIResource(gvk.DaemonSet),
							gvkToAPIResource(gvk.Deployment),
							gvkToAPIResource(gvk.ReplicaSet),
							gvkToAPIResource(gvk.StatefulSet),
						},
					},
					{
						GroupVersion: "batch/v1",
						APIResources: []metav1.APIResource{
							gvkToAPIResource(gvk.Job),
						},
					},
					{
						GroupVersion: "autoscaling/v2",
						APIResources: []metav1.APIResource{
							gvkToAPIResource(gvk.HorizontalPodAutoscaler),
						},
					},
				}
				return client
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs, logs := observer.New(zap.WarnLevel)
			obsLogger := zap.New(obs)
			rw := &resourceWatcher{
				client:        newFakeClientWithAllResources(),
				logger:        obsLogger,
				metadataStore: metadata.NewStore(),
				config:        &Config{},
			}

			assert.NoError(t, rw.prepareSharedInformerFactory())

			// Make sure no warning or error logs are raised
			assert.Equal(t, 0, logs.Len())
		})
	}
}

func TestSetupInformerForKind(t *testing.T) {
	obs, logs := observer.New(zap.WarnLevel)
	obsLogger := zap.New(obs)
	rw := &resourceWatcher{
		client: newFakeClientWithAllResources(),
		logger: obsLogger,
	}

	factory := informers.NewSharedInformerFactoryWithOptions(rw.client, 0)
	rw.setupInformerForKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "WrongKind"}, factory)

	assert.Equal(t, 1, logs.Len())
	assert.Equal(t, "Could not setup an informer for provided group version kind", logs.All()[0].Entry.Message)
}

func TestSyncMetadataAndEmitEntityEvents(t *testing.T) {
	client := newFakeClientWithAllResources()

	logsConsumer := new(consumertest.LogsSink)

	// Setup k8s resources.
	pods := createPods(t, client, 1)

	origPod := pods[0]
	updatedPod := getUpdatedPod(origPod)

	rw := newResourceWatcher(receivertest.NewNopSettings(metadata.Type), &Config{MetadataCollectionInterval: 2 * time.Hour}, metadata.NewStore())
	rw.entityLogConsumer = logsConsumer

	step1 := time.Now()

	// Make some changes to the pod. Each change should result in an entity event represented
	// as a log record.

	// Pod is created.
	rw.syncMetadataUpdate(nil, rw.objMetadata(origPod))
	step2 := time.Now()

	// Pod is updated.
	rw.syncMetadataUpdate(rw.objMetadata(origPod), rw.objMetadata(updatedPod))
	step3 := time.Now()

	// Pod is updated again, but nothing changed in the pod.
	// Should still result in entity event because they are emitted even
	// if the entity is not changed.
	rw.syncMetadataUpdate(rw.objMetadata(updatedPod), rw.objMetadata(updatedPod))
	step4 := time.Now()

	// Change pod's state back to original
	rw.syncMetadataUpdate(rw.objMetadata(updatedPod), rw.objMetadata(origPod))
	step5 := time.Now()

	// Delete the pod
	rw.syncMetadataUpdate(rw.objMetadata(origPod), nil)
	step6 := time.Now()

	// Must have 5 entity events.
	require.EqualValues(t, 5, logsConsumer.LogRecordCount())

	// Event 1 should contain the initial state of the pod.
	lr := logsConsumer.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	expected := map[string]any{
		"otel.entity.event.type": "entity_state",
		"otel.entity.interval":   int64(7200000), // 2h in milliseconds
		"otel.entity.type":       "k8s.pod",
		"otel.entity.id":         map[string]any{"k8s.pod.uid": "pod0"},
		"otel.entity.attributes": map[string]any{"pod.creation_timestamp": "0001-01-01T00:00:00Z", "k8s.pod.phase": "Unknown", "k8s.namespace.name": "test", "k8s.pod.name": "0"},
	}
	assert.EqualValues(t, expected, lr.Attributes().AsRaw())
	assert.WithinRange(t, lr.Timestamp().AsTime(), step1, step2)

	// Event 2 should contain the updated state of the pod.
	lr = logsConsumer.AllLogs()[1].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs := expected["otel.entity.attributes"].(map[string]any)
	attrs["key"] = "value"
	assert.EqualValues(t, expected, lr.Attributes().AsRaw())
	assert.WithinRange(t, lr.Timestamp().AsTime(), step2, step3)

	// Event 3 should be identical to the previous one since pod state didn't change.
	lr = logsConsumer.AllLogs()[2].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.EqualValues(t, expected, lr.Attributes().AsRaw())
	assert.WithinRange(t, lr.Timestamp().AsTime(), step3, step4)

	// Event 4 should contain the reverted state of the pod.
	lr = logsConsumer.AllLogs()[3].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs = expected["otel.entity.attributes"].(map[string]any)
	delete(attrs, "key")
	assert.EqualValues(t, expected, lr.Attributes().AsRaw())
	assert.WithinRange(t, lr.Timestamp().AsTime(), step4, step5)

	// Event 5 should indicate pod deletion.
	lr = logsConsumer.AllLogs()[4].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	expected = map[string]any{
		"otel.entity.event.type": "entity_delete",
		"otel.entity.id":         map[string]any{"k8s.pod.uid": "pod0"},
	}
	assert.EqualValues(t, expected, lr.Attributes().AsRaw())
	assert.WithinRange(t, lr.Timestamp().AsTime(), step5, step6)
}

func TestObjMetadata(t *testing.T) {
	tests := []struct {
		name          string
		metadataStore *metadata.Store
		resource      any
		want          map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata
	}{
		{
			name:          "Pod and container metadata simple case",
			metadataStore: metadata.NewStore(),
			resource: testutils.NewPodWithContainer(
				"0",
				testutils.NewPodSpecWithContainer("container-name"),
				testutils.NewPodStatusWithContainer("container-name", "container-id"),
			),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-pod-0-uid"): {
					EntityType:    "k8s.pod",
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "test-pod-0-uid",
					Metadata:      allPodMetadata(map[string]string{"k8s.pod.phase": "Succeeded", "k8s.pod.name": "test-pod-0", "k8s.namespace.name": "test-namespace"}),
				},
				experimentalmetricmetadata.ResourceID("container-id"): {
					EntityType:    "container",
					ResourceIDKey: "container.id",
					ResourceID:    "container-id",
					Metadata: map[string]string{
						"container.status":             "running",
						"container.creation_timestamp": "0001-01-01T01:01:01Z",
						"container.image.name":         "container-image-name",
						"container.image.tag":          "latest",
						"k8s.container.name":           "container-name",
						"k8s.pod.name":                 "test-pod-0",
						"k8s.pod.uid":                  "test-pod-0-uid",
						"k8s.namespace.name":           "test-namespace",
						"k8s.node.name":                "test-node",
					},
				},
			},
		},
		{
			name:          "Pod with Owner Reference",
			metadataStore: metadata.NewStore(),
			resource: testutils.WithOwnerReferences([]metav1.OwnerReference{
				{
					Kind: "StatefulSet",
					Name: "test-statefulset-0",
					UID:  "test-statefulset-0-uid",
				},
			}, testutils.NewPodWithContainer("0", &corev1.PodSpec{}, &corev1.PodStatus{Phase: corev1.PodFailed, Reason: "Evicted"})),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-pod-0-uid"): {
					EntityType:    "k8s.pod",
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "test-pod-0-uid",
					Metadata: allPodMetadata(map[string]string{
						"k8s.workload.kind":     "StatefulSet",
						"k8s.workload.name":     "test-statefulset-0",
						"k8s.statefulset.name":  "test-statefulset-0",
						"k8s.statefulset.uid":   "test-statefulset-0-uid",
						"k8s.pod.phase":         "Failed",
						"k8s.pod.status_reason": "Evicted",
						"k8s.pod.name":          "test-pod-0",
						"k8s.namespace.name":    "test-namespace",
					}),
				},
			},
		},
		{
			name: "Pod with Service metadata",
			metadataStore: func() *metadata.Store {
				ms := metadata.NewStore()
				ms.Setup(gvk.Service, &testutils.MockStore{
					Cache: map[string]any{
						"test-namespace/test-service": &corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
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
				})
				return ms
			}(),
			resource: podWithAdditionalLabels(
				map[string]string{"k8s-app": "my-app"},
				testutils.NewPodWithContainer("0", &corev1.PodSpec{}, &corev1.PodStatus{Phase: corev1.PodRunning}),
			),
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-pod-0-uid"): {
					EntityType:    "k8s.pod",
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "test-pod-0-uid",
					Metadata: allPodMetadata(map[string]string{
						"k8s.service.test-service": "",
						"k8s-app":                  "my-app",
						"k8s.pod.phase":            "Running",
						"k8s.namespace.name":       "test-namespace",
						"k8s.pod.name":             "test-pod-0",
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
					EntityType:    "k8s.daemonset",
					ResourceIDKey: "k8s.daemonset.uid",
					ResourceID:    "test-daemonset-1-uid",
					Metadata: map[string]string{
						"k8s.workload.kind":            "DaemonSet",
						"k8s.workload.name":            "test-daemonset-1",
						"daemonset.creation_timestamp": "0001-01-01T00:00:00Z",
						"k8s.namespace.name":           "test-namespace",
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
					EntityType:    "k8s.deployment",
					ResourceIDKey: "k8s.deployment.uid",
					ResourceID:    "test-deployment-1-uid",
					Metadata: map[string]string{
						"k8s.workload.kind":             "Deployment",
						"k8s.workload.name":             "test-deployment-1",
						"k8s.deployment.name":           "test-deployment-1",
						"deployment.creation_timestamp": "0001-01-01T00:00:00Z",
						"k8s.namespace.name":            "test-namespace",
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
					EntityType:    "k8s.hpa",
					ResourceIDKey: "k8s.hpa.uid",
					ResourceID:    "test-hpa-1-uid",
					Metadata: map[string]string{
						"k8s.workload.kind":      "HPA",
						"k8s.workload.name":      "test-hpa-1",
						"hpa.creation_timestamp": "0001-01-01T00:00:00Z",
						"k8s.namespace.name":     "test-namespace",
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
					EntityType:    "k8s.job",
					ResourceIDKey: "k8s.job.uid",
					ResourceID:    "test-job-1-uid",
					Metadata: map[string]string{
						"foo":                    "bar",
						"foo1":                   "",
						"k8s.workload.kind":      "Job",
						"k8s.workload.name":      "test-job-1",
						"job.creation_timestamp": "0001-01-01T00:00:00Z",
						"k8s.namespace.name":     "test-namespace",
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
					EntityType:    "k8s.node",
					ResourceIDKey: "k8s.node.uid",
					ResourceID:    "test-node-1-uid",
					Metadata: map[string]string{
						"foo":                                    "bar",
						"foo1":                                   "",
						"k8s.node.name":                          "test-node-1",
						"node.creation_timestamp":                "0001-01-01T00:00:00Z",
						"k8s.node.condition_disk_pressure":       "false",
						"k8s.node.condition_memory_pressure":     "false",
						"k8s.node.condition_network_unavailable": "false",
						"k8s.node.condition_pid_pressure":        "false",
						"k8s.node.condition_ready":               "true",
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
					EntityType:    "k8s.replicaset",
					ResourceIDKey: "k8s.replicaset.uid",
					ResourceID:    "test-replicaset-1-uid",
					Metadata: map[string]string{
						"foo":                           "bar",
						"foo1":                          "",
						"k8s.workload.kind":             "ReplicaSet",
						"k8s.workload.name":             "test-replicaset-1",
						"replicaset.creation_timestamp": "0001-01-01T00:00:00Z",
						"k8s.namespace.name":            "test-namespace",
					},
				},
			},
		},
		{
			name:          "ReplicationController simple case",
			metadataStore: &metadata.Store{},
			resource: &corev1.ReplicationController{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replicationcontroller-1",
					Namespace: "test-namespace",
					UID:       types.UID("test-replicationcontroller-1-uid"),
				},
			},
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-replicationcontroller-1-uid"): {
					EntityType:    "k8s.replicationcontroller",
					ResourceIDKey: "k8s.replicationcontroller.uid",
					ResourceID:    "test-replicationcontroller-1-uid",
					Metadata: map[string]string{
						"k8s.workload.kind":                        "ReplicationController",
						"k8s.workload.name":                        "test-replicationcontroller-1",
						"replicationcontroller.creation_timestamp": "0001-01-01T00:00:00Z",
						"k8s.namespace.name":                       "test-namespace",
					},
				},
			},
		},
		{
			name:          "Namespace metadata",
			metadataStore: metadata.NewStore(),
			resource: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					UID:               types.UID("test-namespace-uid"),
					Name:              "test-namespace",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: time.Now()},
				},
				Status: corev1.NamespaceStatus{
					Phase: corev1.NamespaceActive,
				},
			},
			want: map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
				experimentalmetricmetadata.ResourceID("test-namespace-uid"): {
					EntityType:    "k8s.namespace",
					ResourceIDKey: "k8s.namespace.uid",
					ResourceID:    "test-namespace-uid",
					Metadata: map[string]string{
						"k8s.namespace.name":               "test-namespace",
						"k8s.namespace.phase":              "active",
						"k8s.namespace.creation_timestamp": time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		observedLogger, _ := observer.New(zapcore.WarnLevel)
		set := receivertest.NewNopSettings(metadata.Type)
		set.TelemetrySettings.Logger = zap.New(observedLogger)
		t.Run(tt.name, func(t *testing.T) {
			dc := &resourceWatcher{metadataStore: tt.metadataStore}

			actual := dc.objMetadata(tt.resource)
			require.Equal(t, len(tt.want), len(actual))

			for key, item := range tt.want {
				got, exists := actual[key]
				require.True(t, exists)
				require.Equal(t, *item, *got)
			}
		})
	}
}

var allPodMetadata = func(metadata map[string]string) map[string]string {
	out := maps.MergeStringMaps(metadata, commonPodMetadata)
	return out
}

func podWithAdditionalLabels(labels map[string]string, pod *corev1.Pod) any {
	if pod.Labels == nil {
		pod.Labels = make(map[string]string, len(labels))
	}

	for k, v := range labels {
		pod.Labels[k] = v
	}

	return pod
}
