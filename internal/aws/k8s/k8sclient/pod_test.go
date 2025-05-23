// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var podArray = []any{
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
			Name:      "kube-proxy-csm88",
			Namespace: "kube-system",
			SelfLink:  "/api/v1/namespaces/kube-system/pods/kube-proxy-csm88",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "75ab40d2-552a-4c05-82c9-0ddcb3008657",
			Name:      "coredns-7554568866-26jdf",
			Namespace: "kube-system",
			SelfLink:  "/api/v1/namespaces/kube-system/pods/coredns-7554568866-26jdf",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "b0280963-d68a-4096-ac56-4ecfbaee37f6",
			Name:      "aws-node-wf7jj",
			Namespace: "kube-system",
			SelfLink:  "/api/v1/namespaces/kube-system/pods/aws-node-wf7jj",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "eb09849f-b3f6-4c2c-ba4a-3e2c6eaf24f4",
			Name:      "cloudwatch-agent-rnjfp",
			Namespace: "default",
			SelfLink:  "/api/v1/namespaces/default/pods/cloudwatch-agent-rnjfp",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "11d078c2-6fed-49c3-83a8-b94915a6451f",
			Name:      "guestbook-qbdv8",
			Namespace: "default",
			SelfLink:  "/api/v1/namespaces/default/pods/guestbook-qbdv8",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "52a76177-8431-400c-95b2-109f9b28b3b1",
			Name:      "kube-proxy-v5l9h",
			Namespace: "kube-system",
			SelfLink:  "/api/v1/namespaces/kube-system/pods/kube-proxy-v5l9h",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "bb003966-4134-4ebf-9be3-dfb1741a1499",
			Name:      "redis-slave-mdjsj",
			Namespace: "default",
			SelfLink:  "/api/v1/namespaces/default/pods/redis-slave-mdjsj",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "440854dd-73e8-4616-b21a-3a63831d27e3",
			Name:      "guestbook-qjqnz",
			Namespace: "default",
			SelfLink:  "/api/v1/namespaces/default/pods/guestbook-qjqnz",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "b79e85a1-1fc8-439c-b3a5-bd854ed29e10",
			Name:      "kube-proxy-h5tsv",
			Namespace: "kube-system",
			SelfLink:  "/api/v1/namespaces/kube-system/pods/kube-proxy-h5tsv",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "f9ecca30-f5de-4809-9b63-79dc8369a0a6",
			Name:      "cloudwatch-agent-ksd26",
			Namespace: "default",
			SelfLink:  "/api/v1/namespaces/default/pods/cloudwatch-agent-ksd26",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "cfc496eb-2e5e-490e-9547-f0fe5399c219",
			Name:      "aws-node-pqxp2",
			Namespace: "kube-system",
			SelfLink:  "/api/v1/namespaces/kube-system/pods/aws-node-pqxp2",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "20d46c94-5341-429d-af0c-55d97314db7b",
			Name:      "cloudwatch-agent-2x7p4",
			Namespace: "default",
			SelfLink:  "/api/v1/namespaces/default/pods/cloudwatch-agent-2x7p4",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
	&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "885c6c56-da31-4a63-b823-eed50172193d",
			Name:      "guestbook-92wmq",
			Namespace: "default",
			SelfLink:  "/api/v1/namespaces/default/pods/guestbook-92wmq",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	},
}

// workaround to avoid "unused" lint errors which test is skipped
var skip = func(t *testing.T, why string) {
	t.Skip(why)
}

func TestPodClient_NamespaceToRunningPodNum(t *testing.T) {
	skip(t, "Flaky test - See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11078")
	setOption := podSyncCheckerOption(&mockReflectorSyncChecker{})

	fakeClientSet := fake.NewSimpleClientset()
	client := newPodClient(fakeClientSet, zap.NewNop(), setOption)
	assert.NoError(t, client.store.Replace(podArray, ""))
	client.refresh()

	expectedMap := map[string]int{
		"kube-system": 6,
		"default":     7,
	}
	resultMap := client.NamespaceToRunningPodNum()
	log.Printf("NamespaceToRunningPodNum (len=%v): %v", len(resultMap), resultMap)
	assert.Equal(t, expectedMap, resultMap)
	client.shutdown()
	assert.True(t, client.stopped)
}

func TestTransformFuncPod(t *testing.T) {
	info, err := transformFuncPod(nil)
	assert.Nil(t, info)
	assert.Error(t, err)
}
