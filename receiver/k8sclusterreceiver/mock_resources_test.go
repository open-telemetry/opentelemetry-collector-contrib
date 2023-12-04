// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	quotav1 "github.com/openshift/api/quota/v1"
	fakeQuota "github.com/openshift/client-go/quota/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	fakeDynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvr"
)

func createPods(t *testing.T, client *fake.Clientset, numPods int) []*corev1.Pod {
	out := make([]*corev1.Pod, 0, numPods)
	for i := 0; i < numPods; i++ {
		p := &corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				UID:       types.UID("pod" + strconv.Itoa(i)),
				Name:      strconv.Itoa(i),
				Namespace: "test",
			},
		}

		createdPod, err := client.CoreV1().Pods(p.Namespace).Create(context.Background(), p, v1.CreateOptions{})
		require.NoError(t, err, "error creating node")
		out = append(out, createdPod)
		time.Sleep(2 * time.Millisecond)
	}
	return out
}

func deletePods(t *testing.T, client *fake.Clientset, numPods int) {
	for i := 0; i < numPods; i++ {
		err := client.CoreV1().Pods("test").Delete(context.Background(), strconv.Itoa(i), v1.DeleteOptions{})
		require.NoError(t, err, "error creating node")
	}

	time.Sleep(2 * time.Millisecond)
}

func createNodes(t *testing.T, client *fake.Clientset, numNodes int) {
	for i := 0; i < numNodes; i++ {
		n := &corev1.Node{
			ObjectMeta: v1.ObjectMeta{
				UID:  types.UID("node" + strconv.Itoa(i)),
				Name: strconv.Itoa(i),
			},
		}
		_, err := client.CoreV1().Nodes().Create(context.Background(), n, v1.CreateOptions{})
		require.NoError(t, err, "error creating node")

		time.Sleep(2 * time.Millisecond)
	}
}

func createClusterQuota(t *testing.T, client *fakeQuota.Clientset, numQuotas int) {
	for i := 0; i < numQuotas; i++ {
		q := &quotav1.ClusterResourceQuota{
			ObjectMeta: v1.ObjectMeta{
				Name: fmt.Sprintf("test-clusterquota-%d", i),
				UID:  types.UID(fmt.Sprintf("test-clusterquota-%d-uid", i)),
			},
			Status: quotav1.ClusterResourceQuotaStatus{
				Total: corev1.ResourceQuotaStatus{
					Hard: corev1.ResourceList{
						"requests.cpu": *resource.NewQuantity(5, resource.DecimalSI),
					},
					Used: corev1.ResourceList{
						"requests.cpu": *resource.NewQuantity(4, resource.DecimalSI),
					},
				},
				Namespaces: quotav1.ResourceQuotasStatusByNamespace{
					{
						Namespace: fmt.Sprintf("ns%d", i),
						Status: corev1.ResourceQuotaStatus{
							Hard: corev1.ResourceList{
								"requests.cpu": *resource.NewQuantity(5, resource.DecimalSI),
							},
							Used: corev1.ResourceList{
								"requests.cpu": *resource.NewQuantity(4, resource.DecimalSI),
							},
						},
					},
				},
			},
		}

		_, err := client.QuotaV1().ClusterResourceQuotas().Create(context.Background(), q, v1.CreateOptions{})
		require.NoError(t, err, "error creating node")
		time.Sleep(2 * time.Millisecond)
	}
}

func createHrq(t *testing.T, client *fakeDynamic.FakeDynamicClient) {
	namespace := "test-namespace"
	unstructuredObject := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": gvk.HierarchicalResourceQuota.Version,
			"kind":       gvk.HierarchicalResourceQuota.Kind,
			"metadata": map[string]any{
				"namespace": namespace,
				"name":      "test-hrq",
				"uid":       "test-hrq-uid",
			},
			"status": map[string]any{
				"hard": map[string]any{
					"requests.cpu":    "1",
					"requests.memory": "1Gi",
				},
			},
		},
	}
	_, err := client.Resource(gvr.HierarchicalResourceQuota).Namespace(namespace).Create(context.Background(), unstructuredObject, v1.CreateOptions{})
	require.NoError(t, err, "error creating HierarchicalResourceQuota")
	time.Sleep(2 * time.Millisecond)
}
