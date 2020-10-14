// Copyright 2020 OpenTelemetry Authors
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
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func newFakeAPIClientset(_ k8sconfig.APIConfig) (kubernetes.Interface, error) {
	return fake.NewSimpleClientset(), nil
}

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

func TestDefaultClientset(t *testing.T) {
	c, err := New(zap.NewNop(), k8sconfig.APIConfig{}, ExtractionRules{}, Filters{}, nil, nil)
	assert.Error(t, err)
	assert.Equal(t, "invalid authType for kubernetes: ", err.Error())
	assert.Nil(t, c)

	c, err = New(zap.NewNop(), k8sconfig.APIConfig{}, ExtractionRules{}, Filters{}, newFakeAPIClientset, nil)
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func TestBadFilters(t *testing.T) {
	c, err := New(
		zap.NewNop(),
		k8sconfig.APIConfig{},
		ExtractionRules{},
		Filters{Fields: []FieldFilter{{Op: selection.Exists}}},
		newFakeAPIClientset,
		NewFakeInformer,
	)
	assert.Error(t, err)
	assert.Nil(t, c)
}

func TestClientStartStop(t *testing.T) {
	c, _ := newTestClient(t)
	ctr := c.informer.GetController()
	require.IsType(t, &FakeController{}, ctr)
	fctr := ctr.(*FakeController)
	require.NotNil(t, fctr)

	done := make(chan struct{})
	assert.False(t, fctr.HasStopped())
	go func() {
		c.Start()
		close(done)
	}()
	c.Stop()
	<-done
	assert.True(t, fctr.HasStopped())
}

func TestConstructorErrors(t *testing.T) {
	er := ExtractionRules{}
	ff := Filters{}
	t.Run("client-provider-call", func(t *testing.T) {
		var gotAPIConfig k8sconfig.APIConfig
		apiCfg := k8sconfig.APIConfig{
			AuthType: "test-auth-type",
		}
		clientProvider := func(c k8sconfig.APIConfig) (kubernetes.Interface, error) {
			gotAPIConfig = c
			return nil, fmt.Errorf("error creating k8s client")
		}
		c, err := New(zap.NewNop(), apiCfg, er, ff, clientProvider, NewFakeInformer)
		assert.Nil(t, c)
		assert.Error(t, err)
		assert.Equal(t, err.Error(), "error creating k8s client")
		assert.Equal(t, apiCfg, gotAPIConfig)
	})
}

func TestPodAdd(t *testing.T) {
	c, _ := newTestClient(t)
	podAddAndUpdateTest(t, c, c.handlePodAdd)
}

func TestPodHostNetwork(t *testing.T) {
	c, _ := newTestClient(t)
	assert.Equal(t, 0, len(c.Pods))

	pod := &api_v1.Pod{}
	pod.Name = "podA"
	pod.Status.PodIP = "1.1.1.1"
	pod.Spec.HostNetwork = true
	c.handlePodAdd(pod)
	assert.Equal(t, len(c.Pods), 1)
	got := c.Pods["1.1.1.1"]
	assert.Equal(t, got.Address, "1.1.1.1")
	assert.Equal(t, got.Name, "podA")
	assert.True(t, got.Ignore)
}

func TestPodAddOutOfSync(t *testing.T) {
	c, _ := newTestClient(t)
	assert.Equal(t, len(c.Pods), 0)

	pod := &api_v1.Pod{}
	pod.Name = "podA"
	pod.Status.PodIP = "1.1.1.1"
	startTime := meta_v1.NewTime(time.Now())
	pod.Status.StartTime = &startTime
	c.handlePodAdd(pod)
	assert.Equal(t, len(c.Pods), 1)
	got := c.Pods["1.1.1.1"]
	assert.Equal(t, got.Address, "1.1.1.1")
	assert.Equal(t, got.Name, "podA")

	pod2 := &api_v1.Pod{}
	pod2.Name = "podB"
	pod.Status.PodIP = "1.1.1.1"
	startTime2 := meta_v1.NewTime(time.Now().Add(-time.Second * 10))
	pod.Status.StartTime = &startTime2
	c.handlePodAdd(pod)
	assert.Equal(t, len(c.Pods), 1)
	got = c.Pods["1.1.1.1"]
	assert.Equal(t, got.Address, "1.1.1.1")
	assert.Equal(t, got.Name, "podA")
}

func TestPodUpdate(t *testing.T) {
	c, _ := newTestClient(t)
	podAddAndUpdateTest(t, c, func(obj interface{}) {
		// first argument (old pod) is not used right now
		c.handlePodUpdate(&api_v1.Pod{}, obj)
	})
}

func TestPodDelete(t *testing.T) {
	c, _ := newTestClient(t)
	podAddAndUpdateTest(t, c, c.handlePodAdd)
	assert.Equal(t, len(c.Pods), 1)
	assert.Equal(t, c.Pods["1.1.1.1"].Address, "1.1.1.1")

	// delete empty IP pod
	c.handlePodDelete(&api_v1.Pod{})

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
	assert.False(t, deleteRequest.ts.Before(tsBeforeDelete))
	assert.False(t, deleteRequest.ts.After(time.Now()))
}

func TestDeleteQueue(t *testing.T) {
	c, _ := newTestClient(t)
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
}

func TestDeleteLoop(t *testing.T) {
	// go c.deleteLoop(time.Second * 1)
	c, _ := newTestClient(t)

	pod := &api_v1.Pod{}
	pod.Status.PodIP = "1.1.1.1"
	c.handlePodAdd(pod)
	assert.Equal(t, len(c.Pods), 1)
	assert.Equal(t, len(c.deleteQueue), 0)

	c.handlePodDelete(pod)
	assert.Equal(t, len(c.Pods), 1)
	assert.Equal(t, len(c.deleteQueue), 1)

	gracePeriod := time.Millisecond * 500
	go c.deleteLoop(time.Millisecond, gracePeriod)
	go func() {
		time.Sleep(time.Millisecond * 50)
		c.m.Lock()
		assert.Equal(t, len(c.Pods), 1)
		c.m.Unlock()
		c.deleteMut.Lock()
		assert.Equal(t, len(c.deleteQueue), 1)
		c.deleteMut.Unlock()

		time.Sleep(gracePeriod + (time.Millisecond * 50))
		c.m.Lock()
		assert.Equal(t, len(c.Pods), 0)
		c.m.Unlock()
		c.deleteMut.Lock()
		assert.Equal(t, len(c.deleteQueue), 0)
		c.deleteMut.Unlock()
		close(c.stopCh)
	}()
	<-c.stopCh
}

func TestGetIgnoredPod(t *testing.T) {
	c, _ := newTestClient(t)
	pod := &api_v1.Pod{}
	pod.Status.PodIP = "1.1.1.1"
	c.handlePodAdd(pod)
	c.Pods[pod.Status.PodIP].Ignore = true
	got, ok := c.GetPodByIP(pod.Status.PodIP)
	assert.Nil(t, got)
	assert.False(t, ok)
}

func TestHandlerWrongType(t *testing.T) {
	c, logs := newTestClientWithRulesAndFilters(t, ExtractionRules{}, Filters{})
	assert.Equal(t, logs.Len(), 0)
	c.handlePodAdd(1)
	c.handlePodDelete(1)
	c.handlePodUpdate(1, 2)
	assert.Equal(t, logs.Len(), 3)
	for _, l := range logs.All() {
		assert.Equal(t, l.Message, "object received was not of type api_v1.Pod")
	}
}

func TestExtractionRules(t *testing.T) {
	c, _ := newTestClientWithRulesAndFilters(t, ExtractionRules{}, Filters{})

	pod := &api_v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "auth-service-abc12-xyz3",
			UID:               "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
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
		},
		Spec: api_v1.PodSpec{
			NodeName: "node1",
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
		},
		attributes: map[string]string{
			"k8s.deployment.name": "auth-service",
		},
	}, {
		name: "metadata",
		rules: ExtractionRules{
			Deployment: true,
			Namespace:  true,
			PodName:    true,
			PodUID:     true,
			Node:       true,
			Cluster:    true,
			StartTime:  true,
		},
		attributes: map[string]string{
			"k8s.deployment.name": "auth-service",
			"k8s.namespace.name":  "ns1",
			"k8s.cluster.name":    "cluster1",
			"k8s.node.name":       "node1",
			"k8s.pod.name":        "auth-service-abc12-xyz3",
			"k8s.pod.uid":         "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			"k8s.pod.startTime":   pod.GetCreationTimestamp().String(),
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
			c, _ := newTestClientWithRulesAndFilters(t, ExtractionRules{}, tc.filters)
			inf := c.informer.(*FakeInformer)
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

	c, _ := newTestClient(t)
	for _, tc := range testCases {
		assert.Equal(t, tc.ignore, c.shouldIgnorePod(&tc.pod))
	}
}

func Test_extractField(t *testing.T) {
	c := WatchClient{}
	type args struct {
		v string
		r FieldExtractionRule
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"no-regex",
			args{
				"str",
				FieldExtractionRule{Regex: nil},
			},
			"str",
		},
		{
			"basic",
			args{
				"str",
				FieldExtractionRule{Regex: regexp.MustCompile("field=(?P<value>.+)")},
			},
			"",
		},
		{
			"basic",
			args{
				"field=val1",
				FieldExtractionRule{Regex: regexp.MustCompile("field=(?P<value>.+)")},
			},
			"val1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.extractField(tt.args.v, tt.args.r); got != tt.want {
				t.Errorf("extractField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_selectorsFromFilters(t *testing.T) {
	tests := []struct {
		name    string
		filters Filters
		wantL   labels.Selector
		wantF   fields.Selector
		wantErr bool
	}{
		{
			"label/invalid-op",
			Filters{
				Labels: []FieldFilter{{Op: "invalid-op"}},
			},
			nil,
			nil,
			true,
		},
		{
			"fields/invalid-op",
			Filters{
				Fields: []FieldFilter{{Op: selection.Exists}},
			},
			nil,
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := selectorsFromFilters(tt.filters)
			if (err != nil) != tt.wantErr {
				t.Errorf("selectorsFromFilters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.wantL) {
				t.Errorf("selectorsFromFilters() got = %v, want %v", got, tt.wantL)
			}
			if !reflect.DeepEqual(got1, tt.wantF) {
				t.Errorf("selectorsFromFilters() got1 = %v, want %v", got1, tt.wantF)
			}
		})
	}
}

func newTestClientWithRulesAndFilters(t *testing.T, e ExtractionRules, f Filters) (*WatchClient, *observer.ObservedLogs) {
	observedLogger, logs := observer.New(zapcore.WarnLevel)
	logger := zap.New(observedLogger)
	c, err := New(logger, k8sconfig.APIConfig{}, e, f, newFakeAPIClientset, NewFakeInformer)
	require.NoError(t, err)
	return c.(*WatchClient), logs
}

func newTestClient(t *testing.T) (*WatchClient, *observer.ObservedLogs) {
	return newTestClientWithRulesAndFilters(t, ExtractionRules{}, Filters{})
}
