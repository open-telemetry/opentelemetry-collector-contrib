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
				meta_v1.OwnerReference{
					Kind: "somekind",
					Name: "somename",
				},
			},
		},
		Spec: api_v1.PodSpec{
			NodeName: "node1",
			Hostname: "auth-hostname3",
		},
		Status: api_v1.PodStatus{
			PodIP: "1.1.1.1",
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
			Deployment:  true,
			Namespace:   true,
			PodName:     true,
			NodeName:    true,
			ClusterName: true,
			StartTime:   true,
			HostName:    true,
			Owners:      true,
			Tags:        NewExtractionFieldTags(),
		},
		attributes: map[string]string{
			"k8s.deployment.name": "auth-service",
			"k8s.namespace.name":  "ns1",
			"k8s.cluster.name":    "cluster1",
			"k8s.node.name":       "node1",
			"k8s.pod.name":        "auth-service-abc12-xyz3",
			"k8s.pod.startTime":   pod.GetCreationTimestamp().String(),
			"k8s.pod.hostname":    "auth-hostname3",
			"k8s.owner.somekind":  "somename",
		},
	}, {
		name: "non-default tags",
		rules: ExtractionRules{
			Deployment:  true,
			Namespace:   true,
			PodName:     true,
			NodeName:    true,
			ClusterName: true,
			StartTime:   true,
			HostName:    true,
			Owners:      true,
			Tags: ExtractionFieldTags{
				Deployment:    "d",
				Namespace:     "n",
				PodName:       "p",
				NodeName:      "nn",
				ClusterName:   "cc",
				StartTime:     "st",
				HostName:      "hn",
				OwnerTemplate: "ow-%s",
			},
		},
		attributes: map[string]string{
			"d":           "auth-service",
			"n":           "ns1",
			"cc":          "cluster1",
			"nn":          "node1",
			"p":           "auth-service-abc12-xyz3",
			"st":          pod.GetCreationTimestamp().String(),
			"hn":          "auth-hostname3",
			"ow-somekind": "somename",
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
	c, err := New(zap.NewNop(), e, f, newFakeAPIClientset, newFakeInformer)
	require.NoError(t, err)
	return c.(*WatchClient)
}

func newTestClient(t *testing.T) *WatchClient {
	return newTestClientWithRulesAndFilters(t, ExtractionRules{}, Filters{})
}
