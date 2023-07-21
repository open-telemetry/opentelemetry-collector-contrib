// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/collection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
)

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
					component.NewID("nop"): MockExporter{},
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
			args{exporters: map[component.ID]component.Component{
				component.NewID("nop"): mockExporterWithK8sMetadata{},
			},
				metadataExportersFromConfig: []string{"nop"},
			},
			false,
		},
		{
			"Non-existent exporter",
			fields{
				metadataConsumers: []metadataConsumer{},
			},
			args{exporters: map[component.ID]component.Component{
				component.NewID("nop"): mockExporterWithK8sMetadata{},
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
	var tests = []struct {
		name     string
		client   *fake.Clientset
		gvk      schema.GroupVersionKind
		expected bool
	}{
		{
			name:     "nothing_supported",
			client:   fake.NewSimpleClientset(),
			gvk:      gvk.Pod,
			expected: false,
		},
		{
			name:     "all_kinds_supported",
			client:   newFakeClientWithAllResources(),
			gvk:      gvk.Pod,
			expected: true,
		},
		{
			name:     "unsupported_kind",
			client:   fake.NewSimpleClientset(),
			gvk:      gvk.CronJobBeta,
			expected: false,
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
	var tests = []struct {
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
				client := fake.NewSimpleClientset()
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
						GroupVersion: "batch/v1beta1",
						APIResources: []metav1.APIResource{
							gvkToAPIResource(gvk.CronJobBeta),
						},
					},
					{
						GroupVersion: "autoscaling/v2",
						APIResources: []metav1.APIResource{
							gvkToAPIResource(gvk.HorizontalPodAutoscaler),
						},
					},
					{
						GroupVersion: "autoscaling/v2beta2",
						APIResources: []metav1.APIResource{
							gvkToAPIResource(gvk.HorizontalPodAutoscalerBeta),
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
				dataCollector: collection.NewDataCollector(receivertest.NewNopCreateSettings(), []string{}, []string{}),
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

	rw := newResourceWatcher(receivertest.NewNopCreateSettings(), &Config{})
	rw.entityLogConsumer = logsConsumer

	// Make some changes to the pod. Each change should result in an entity event represented
	// as a log record.

	// Pod is created.
	rw.syncMetadataUpdate(nil, rw.dataCollector.SyncMetadata(origPod))

	// Pod is updated.
	rw.syncMetadataUpdate(rw.dataCollector.SyncMetadata(origPod), rw.dataCollector.SyncMetadata(updatedPod))

	// Pod is updated again, but nothing changed in the pod.
	// Should still result in entity event because they are emitted even
	// if the entity is not changed.
	rw.syncMetadataUpdate(rw.dataCollector.SyncMetadata(updatedPod), rw.dataCollector.SyncMetadata(updatedPod))

	// Change pod's state back to original
	rw.syncMetadataUpdate(rw.dataCollector.SyncMetadata(updatedPod), rw.dataCollector.SyncMetadata(origPod))

	// Delete the pod
	rw.syncMetadataUpdate(rw.dataCollector.SyncMetadata(origPod), nil)

	// Must have 5 entity events.
	require.EqualValues(t, 5, logsConsumer.LogRecordCount())

	// Event 1 should contain the initial state of the pod.
	lr := logsConsumer.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	expected := map[string]any{
		"otel.entity.event.type": "entity_state",
		"otel.entity.type":       "k8s.pod",
		"otel.entity.id":         map[string]any{"k8s.pod.uid": "pod0"},
		"otel.entity.attributes": map[string]any{"pod.creation_timestamp": "0001-01-01T00:00:00Z"},
	}
	assert.EqualValues(t, expected, lr.Attributes().AsRaw())

	// Event 2 should contain the updated state of the pod.
	lr = logsConsumer.AllLogs()[1].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs := expected["otel.entity.attributes"].(map[string]any)
	attrs["key"] = "value"
	assert.EqualValues(t, expected, lr.Attributes().AsRaw())

	// Event 3 should be identical to the previous one since pod state didn't change.
	lr = logsConsumer.AllLogs()[2].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.EqualValues(t, expected, lr.Attributes().AsRaw())

	// Event 4 should contain the reverted state of the pod.
	lr = logsConsumer.AllLogs()[3].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs = expected["otel.entity.attributes"].(map[string]any)
	delete(attrs, "key")
	assert.EqualValues(t, expected, lr.Attributes().AsRaw())

	// Event 5 should indicate pod deletion.
	lr = logsConsumer.AllLogs()[4].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	expected = map[string]any{
		"otel.entity.event.type": "entity_delete",
		"otel.entity.id":         map[string]any{"k8s.pod.uid": "pod0"},
	}
	assert.EqualValues(t, expected, lr.Attributes().AsRaw())
}
