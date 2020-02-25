// Copyright 2019 Omnition Authors
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

package kube

import (
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
)

func podAddAndUpdateTest(t *testing.T, c *WatchClient, handler func(obj interface{})) {
	assert.Equal(t, len(c.Pods), 0)

	// pod without IP
	pod := &api_v1.Pod{}
	handler(pod)
	assert.Equal(t, len(c.Pods), 0)

	pod = &api_v1.Pod{}
	pod.Name = "podA"
	pod.Status.PodIP = "1.1.1.1"
	handler(pod)
	assert.Equal(t, len(c.Pods), 1)
	got := c.Pods["1.1.1.1"]
	assert.Equal(t, got.Address, "1.1.1.1")
	assert.Equal(t, got.Name, "podA")

	pod = &api_v1.Pod{}
	pod.Name = "podB"
	pod.Status.PodIP = "1.1.1.1"
	handler(pod)
	assert.Equal(t, len(c.Pods), 1)
	got = c.Pods["1.1.1.1"]
	assert.Equal(t, got.Address, "1.1.1.1")
	assert.Equal(t, got.Name, "podB")
}

func TestPodAdd(t *testing.T) {
	c := newTestClient(t)
	podAddAndUpdateTest(t, c, c.handlePodAdd)
}

func TestPodUpdate(t *testing.T) {
	c := newTestClient(t)
	podAddAndUpdateTest(t, c, func(obj interface{}) {
		// first argument (old pod) is not used right now
		c.handlePodUpdate(&api_v1.Pod{}, obj)
	})
}

func TestPodDelete(t *testing.T) {
	c := newTestClient(t)
	podAddAndUpdateTest(t, c, c.handlePodAdd)
	assert.Equal(t, len(c.Pods), 1)
	assert.Equal(t, c.Pods["1.1.1.1"].Address, "1.1.1.1")

	// delete non-existent IP
	pod := &api_v1.Pod{}
	pod.Status.PodIP = "9.9.9.9"
	c.handlePodDelete(pod)
	assert.Equal(t, len(c.Pods), 1)
	got := c.Pods["1.1.1.1"]
	assert.Equal(t, got.Address, "1.1.1.1")
	assert.Equal(t, len(c.deleteQueue), 0)

	// delete matching IP with wrong name/different pod
	pod = &api_v1.Pod{}
	pod.Status.PodIP = "1.1.1.1"
	c.handlePodDelete(pod)
	got = c.Pods["1.1.1.1"]
	assert.Equal(t, len(c.Pods), 1)
	assert.Equal(t, got.Address, "1.1.1.1")
	assert.Equal(t, len(c.deleteQueue), 0)

	// delete matching IP and name
	pod = &api_v1.Pod{}
	pod.Name = "podB"
	pod.Status.PodIP = "1.1.1.1"
	tsBeforeDelete := time.Now()
	c.handlePodDelete(pod)
	assert.Equal(t, len(c.Pods), 1)
	assert.Equal(t, len(c.deleteQueue), 1)
	deleteRequest := c.deleteQueue[0]
	assert.Equal(t, deleteRequest.ip, "1.1.1.1")
	assert.Equal(t, deleteRequest.name, "podB")
	assert.True(t, deleteRequest.ts.After(tsBeforeDelete))
	assert.True(t, deleteRequest.ts.Before(time.Now()))
}

func TestDeleteQueue(t *testing.T) {
	c := newTestClient(t)
	podAddAndUpdateTest(t, c, c.handlePodAdd)
	assert.Equal(t, len(c.Pods), 1)
	assert.Equal(t, c.Pods["1.1.1.1"].Address, "1.1.1.1")

	// delete pod
	pod := &api_v1.Pod{}
	pod.Name = "podB"
	pod.Status.PodIP = "1.1.1.1"
	c.handlePodDelete(pod)
	assert.Equal(t, len(c.Pods), 1)
	assert.Equal(t, len(c.deleteQueue), 1)

	// test delete loop
}

func TestExtractionRules(t *testing.T) {
	c := newTestClientWithRulesAndFilters(t, ExtractionRules{}, Filters{})

	pod := &api_v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "auth-service-abc12-xyz3",
			Namespace:         "ns1",
			UID:               "33333",
			CreationTimestamp: meta_v1.Now(),
			ClusterName:       "cluster1",
			Labels: map[string]string{
				"label1": "lv1",
				"label2": "k1=v1 k5=v5 extra!",
			},
			Annotations: map[string]string{
				"annotation1": "av1",
			},
			OwnerReferences: []meta_v1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "foo-bar-rs",
					UID:  "1a1658f9-7818-11e9-90f1-02324f7e0d1e",
				},
			},
		},
		Spec: api_v1.PodSpec{
			NodeName: "node1",
			Hostname: "auth-hostname3",
			Containers: []api_v1.Container{
				{
					Image: "auth-service-image",
					Name:  "auth-service-container-name",
				},
			},
		},
		Status: api_v1.PodStatus{
			PodIP: "1.1.1.1",
			ContainerStatuses: []api_v1.ContainerStatus{
				{
					ContainerID: "111-222-333",
				},
			},
		},
	}

	testCases := []struct {
		name       string
		rules      ExtractionRules
		attributes map[string]string
	}{{
		name:       "no-rules",
		rules:      ExtractionRules{},
		attributes: nil,
	}, {
		name: "deployment",
		rules: ExtractionRules{
			Deployment: true,
			Tags:       NewExtractionFieldTags(),
		},
		attributes: map[string]string{
			"k8s.deployment.name": "auth-service",
		},
	}, {
		name: "metadata",
		rules: ExtractionRules{
			ClusterName:     true,
			ContainerID:     true,
			ContainerImage:  true,
			ContainerName:   true,
			DaemonSetName:   true,
			Deployment:      true,
			HostName:        true,
			Owners:          true,
			PodID:           true,
			PodName:         true,
			ReplicaSetName:  true,
			ServiceName:     true,
			StatefulSetName: true,
			StartTime:       true,
			Namespace:       true,
			NamespaceID:     true,
			NodeName:        true,
			Tags:            NewExtractionFieldTags(),
		},
		attributes: map[string]string{
			"k8s.cluster.name":    "cluster1",
			"k8s.container.id":    "111-222-333",
			"k8s.container.image": "auth-service-image",
			"k8s.container.name":  "auth-service-container-name",
			"k8s.deployment.name": "auth-service",
			"k8s.pod.hostname":    "auth-hostname3",
			"k8s.pod.id":          "33333",
			"k8s.pod.name":        "auth-service-abc12-xyz3",
			"k8s.pod.startTime":   pod.GetCreationTimestamp().String(),
			"k8s.replicaset.name": "SomeReplicaSet",
			"k8s.namespace.name":  "ns1",
			"k8s.namespace.id":    "33333-66666",
			"k8s.node.name":       "node1",
		},
	}, {
		name: "non-default tags",
		rules: ExtractionRules{
			ClusterName:     true,
			ContainerID:     true,
			ContainerImage:  false,
			ContainerName:   true,
			DaemonSetName:   false,
			Deployment:      false,
			HostName:        false,
			Owners:          false,
			PodID:           false,
			PodName:         false,
			ReplicaSetName:  false,
			ServiceName:     false,
			StatefulSetName: false,
			StartTime:       false,
			Namespace:       false,
			NamespaceID:     false,
			NodeName:        false,
			Tags: ExtractionFieldTags{
				ClusterName:   "cc",
				ContainerID:   "cid",
				ContainerName: "cn",
			},
		},
		attributes: map[string]string{
			"cc":  "cluster1",
			"cid": "111-222-333",
			"cn":  "auth-service-container-name",
		},
	}, {
		name: "labels",
		rules: ExtractionRules{
			Annotations: []FieldExtractionRule{{
				Name: "a1",
				Key:  "annotation1",
			},
			},
			Labels: []FieldExtractionRule{{
				Name: "l1",
				Key:  "label1",
			}, {
				Name:  "l2",
				Key:   "label2",
				Regex: regexp.MustCompile(`k5=(?P<value>[^\s]+)`),
			},
			},
		},
		attributes: map[string]string{
			"l1": "lv1",
			"l2": "v5",
			"a1": "av1",
		},
	},
		{
			name: "generic-labels",
			rules: ExtractionRules{
				Tags: NewExtractionFieldTags(),
				Annotations: []FieldExtractionRule{{
					Name: "k8s.pod.annotation.%s",
					Key:  "*",
				},
				},
				Labels: []FieldExtractionRule{{
					Name: "k8s.pod.label.%s",
					Key:  "*",
				},
				},
			},
			attributes: map[string]string{
				"k8s.pod.label.label1":           "lv1",
				"k8s.pod.label.label2":           "k1=v1 k5=v5 extra!",
				"k8s.pod.annotation.annotation1": "av1",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c.Rules = tc.rules
			c.handlePodAdd(pod)
			p, ok := c.GetPodByIP(pod.Status.PodIP)
			require.True(t, ok)

			assert.Equal(t, len(tc.attributes), len(p.Attributes))
			for k, v := range tc.attributes {
				got, ok := p.Attributes[k]
				assert.True(t, ok)
				assert.Equal(t, v, got)
			}
		})
	}
}

func TestFilters(t *testing.T) {
	testCases := []struct {
		name    string
		filters Filters
		labels  string
		fields  string
	}{{
		name:    "no-filters",
		filters: Filters{},
	}, {
		name: "namespace",
		filters: Filters{
			Namespace: "default",
		},
	}, {
		name: "node",
		filters: Filters{
			Node: "ec2-test",
		},
		fields: "spec.nodeName=ec2-test",
	}, {
		name: "labels-and-fields",
		filters: Filters{
			Labels: []FieldFilter{
				{
					Key:   "k1",
					Value: "v1",
					Op:    selection.Equals,
				},
				{
					Key:   "k2",
					Value: "v2",
					Op:    selection.NotEquals,
				},
			},
			Fields: []FieldFilter{
				{
					Key:   "k1",
					Value: "v1",
					Op:    selection.Equals,
				},
				{
					Key:   "k2",
					Value: "v2",
					Op:    selection.NotEquals,
				},
			},
		},
		labels: "k1=v1,k2!=v2",
		fields: "k1=v1,k2!=v2",
	},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClientWithRulesAndFilters(t, ExtractionRules{}, tc.filters)
			inf := c.informer.(fakeInformer)
			assert.Equal(t, tc.filters.Namespace, inf.namespace)
			assert.Equal(t, tc.labels, inf.labelSelector.String())
			assert.Equal(t, tc.fields, inf.fieldSelector.String())
		})
	}

}

func TestPodIgnorePatterns(t *testing.T) {
	testCases := []struct {
		ignore bool
		pod    api_v1.Pod
	}{{
		ignore: false,
		pod:    api_v1.Pod{},
	}, {
		ignore: true,
		pod: api_v1.Pod{
			Spec: api_v1.PodSpec{
				HostNetwork: true,
			},
		},
	}, {
		ignore: true,
		pod: api_v1.Pod{
			ObjectMeta: meta_v1.ObjectMeta{
				Annotations: map[string]string{
					"opentelemetry.io/k8s-processor/ignore": "True ",
				},
			},
		},
	}, {
		ignore: true,
		pod: api_v1.Pod{
			ObjectMeta: meta_v1.ObjectMeta{
				Annotations: map[string]string{
					"opentelemetry.io/k8s-processor/ignore": "true",
				},
			},
		},
	}, {
		ignore: false,
		pod: api_v1.Pod{
			ObjectMeta: meta_v1.ObjectMeta{
				Annotations: map[string]string{
					"opentelemetry.io/k8s-processor/ignore": "false",
				},
			},
		},
	}, {
		ignore: false,
		pod: api_v1.Pod{
			ObjectMeta: meta_v1.ObjectMeta{
				Annotations: map[string]string{
					"opentelemetry.io/k8s-processor/ignore": "",
				},
			},
		},
	}, {
		ignore: true,
		pod: api_v1.Pod{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: "jaeger-agent",
			},
		},
	}, {
		ignore: true,
		pod: api_v1.Pod{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: "jaeger-collector",
			},
		},
	},
	}

	c := newTestClient(t)
	for _, tc := range testCases {
		assert.Equal(t, tc.ignore, c.shouldIgnorePod(&tc.pod))
	}
}

func newTestClientWithRulesAndFilters(t *testing.T, e ExtractionRules, f Filters) *WatchClient {
	c, err := New(zap.NewNop(), e, f, newFakeAPIClientset, newFakeInformer, newFakeOwnerProvider)
	require.NoError(t, err)
	return c.(*WatchClient)
}

func newTestClient(t *testing.T) *WatchClient {
	return newTestClientWithRulesAndFilters(t, ExtractionRules{}, Filters{})
}

func newBenchmarkClient(b *testing.B) *WatchClient {
	e := ExtractionRules{
		ClusterName:     true,
		ContainerID:     true,
		ContainerImage:  true,
		ContainerName:   true,
		DaemonSetName:   true,
		Deployment:      true,
		HostName:        true,
		Owners:          true,
		PodID:           true,
		PodName:         true,
		ReplicaSetName:  true,
		ServiceName:     true,
		StatefulSetName: true,
		StartTime:       true,
		Namespace:       true,
		NamespaceID:     true,
		NodeName:        true,
		Tags:            NewExtractionFieldTags(),
	}
	f := Filters{}

	c, _ := New(zap.NewNop(), e, f, newFakeAPIClientset, newFakeInformer, newFakeOwnerProvider)
	return c.(*WatchClient)
}

// benchmark actually checks what's the impact of adding new Pod, which is mostly impacted by duration of API call
func benchmark(b *testing.B, podsPerUniqueOwner int) {
	c := newBenchmarkClient(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pod := &api_v1.Pod{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:              fmt.Sprintf("pod-number-%d", i),
				Namespace:         "ns1",
				UID:               types.UID(fmt.Sprintf("33333-%d", i)),
				CreationTimestamp: meta_v1.Now(),
				ClusterName:       "cluster1",
				Labels: map[string]string{
					"label1": fmt.Sprintf("lv1-%d", i),
					"label2": "k1=v1 k5=v5 extra!",
				},
				Annotations: map[string]string{
					"annotation1": fmt.Sprintf("av%d", i),
				},
				OwnerReferences: []meta_v1.OwnerReference{
					{
						Kind: "ReplicaSet",
						Name: "foo-bar-rs",
						UID:  types.UID(fmt.Sprintf("1a1658f9-7818-11e9-90f1-02324f7e0d1e-%d", i/podsPerUniqueOwner)),
					},
				},
			},
			Spec: api_v1.PodSpec{
				NodeName: "node1",
				Hostname: "auth-hostname3",
				Containers: []api_v1.Container{
					{
						Image: "auth-service-image",
						Name:  "auth-service-container-name",
					},
				},
			},
			Status: api_v1.PodStatus{
				PodIP: fmt.Sprintf("%d.%d.%d.%d", (i>>24)%256, (i>>16)%256, (i>>8)%256, i%256),
				ContainerStatuses: []api_v1.ContainerStatus{
					{
						ContainerID: fmt.Sprintf("111-222-333-%d", i),
					},
				},
			},
		}

		c.handlePodAdd(pod)
		_, ok := c.GetPodByIP(pod.Status.PodIP)
		require.True(b, ok)

	}

}

func BenchmarkManyPodsPerOwner(b *testing.B) {
	benchmark(b, 100000)
}

func BenchmarkFewPodsPerOwner(b *testing.B) {
	benchmark(b, 10)
}
