// Copyright The OpenTelemetry Authors
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

package k8smetadata

import (
	"context"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMetadataCache(t *testing.T) {
	m := MetadataCache{}
	entry := MetadataCacheEntry{
		Labels: map[string]string{
			"label1": "value1",
		},
		Annotations: map[string]string{
			"annotation2": "value2",
		},
	}

	m.Store("testkey", entry)
	loaded, ok := m.Load("testkey")
	require.True(t, ok)
	require.Equal(t, entry, loaded)
}

func basicConfig() *K8sMetadataDecoratorConfig {
	cfg := NewK8sMetadataDecoratorConfig("testoperator")
	cfg.OutputIDs = []string{"mock"}
	return cfg
}

func TestK8sMetadataDecoratorBuildDefault(t *testing.T) {
	cfg := basicConfig()

	expected := &K8sMetadataDecorator{
		TransformerOperator: helper.TransformerOperator{
			WriterOperator: helper.WriterOperator{
				BasicOperator: helper.BasicOperator{
					OperatorID:   "$.testoperator",
					OperatorType: "k8s_metadata_decorator",
				},
				OutputIDs: []string{"$.mock"},
			},
			OnError: "send",
		},
		podNameField:   entry.NewResourceField("k8s.pod.name"),
		namespaceField: entry.NewResourceField("k8s.namespace.name"),
		cacheTTL:       10 * time.Minute,
		timeout:        10 * time.Second,
		allowProxy:     false,
	}

	ops, err := cfg.Build(testutil.NewBuildContext(t))
	require.NoError(t, err)
	op := ops[0]
	op.(*K8sMetadataDecorator).SugaredLogger = nil
	require.Equal(t, expected, op)

}

func TestK8sMetadataDecoratorCachedMetadata(t *testing.T) {

	cfg := basicConfig()
	ops, err := cfg.Build(testutil.NewBuildContext(t))
	require.NoError(t, err)
	op := ops[0]

	mockOutput := testutil.NewMockOperator("mock")
	op.SetOutputs([]operator.Operator{mockOutput})

	// Preload cache so we don't hit the network
	k8s := op.(*K8sMetadataDecorator)
	k8s.namespaceCache.Store("testnamespace", MetadataCacheEntry{
		ExpirationTime: time.Now().Add(time.Hour),
		Labels: map[string]string{
			"label1": "lab1",
		},
		ClusterName: "testcluster",
		Annotations: map[string]string{
			"annotation1": "ann1",
		},
	})

	k8s.podCache.Store("testnamespace:testpodname", MetadataCacheEntry{
		ExpirationTime: time.Now().Add(time.Hour),
		Labels: map[string]string{
			"podlabel1": "podlab1",
		},
		Annotations: map[string]string{
			"podannotation1": "podann1",
		},
		AdditionalResourceValues: map[string]string{
			"k8s.service.name": "testservice",
		},
	})

	expected := entry.Entry{
		Labels: map[string]string{
			"k8s-pod/podlabel1":                 "podlab1",
			"k8s-ns/label1":                     "lab1",
			"k8s-pod-annotation/podannotation1": "podann1",
			"k8s-ns-annotation/annotation1":     "ann1",
		},
		Resource: map[string]string{
			"k8s.pod.name":       "testpodname",
			"k8s.namespace.name": "testnamespace",
			"k8s.service.name":   "testservice",
			"k8s.cluster.name":   "testcluster",
		},
	}

	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		entry := args.Get(1).(*entry.Entry)
		require.Equal(t, expected.Labels, entry.Labels)
		require.Equal(t, expected.Resource, entry.Resource)
	}).Return(nil)

	e := &entry.Entry{
		Resource: map[string]string{
			"k8s.pod.name":       "testpodname",
			"k8s.namespace.name": "testnamespace",
		},
	}
	err = op.Process(context.Background(), e)
	require.NoError(t, err)
}
