// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sclusterreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
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
				dataCollector: collection.NewDataCollector(zap.NewNop(), []string{}, []string{}),
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
