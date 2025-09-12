// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

// createTestIngress creates a test ingress object with the given name and namespace
func createTestIngress(name, namespace string) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(name + "-uid"),
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: name + ".example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/",
									PathType: func() *networkingv1.PathType {
										pt := networkingv1.PathTypePrefix
										return &pt
									}(),
								},
							},
						},
					},
				},
			},
		},
	}
}

var ingressObjects = []runtime.Object{
	createTestIngress("ingress-1", "default"),
	createTestIngress("ingress-2", "kube-system"),
	createTestIngress("ingress-3", "default"),
}

func TestIngressClient_GetIngressMetrics(t *testing.T) {
	setOption := ingressSyncCheckerOption(&mockReflectorSyncChecker{})
	fakeClientSet := fake.NewSimpleClientset(ingressObjects...)
	client, err := newIngressClient(fakeClientSet, zap.NewNop(), setOption)
	assert.NoError(t, err)

	ingresses := make([]any, len(ingressObjects))
	for i := range ingressObjects {
		ingresses[i] = ingressObjects[i]
	}
	assert.NoError(t, client.store.Replace(ingresses, ""))

	metrics := client.GetIngressMetrics()
	assert.NotNil(t, metrics)
	assert.Len(t, metrics.NamespaceCount, 2)
	assert.Equal(t, 2, metrics.NamespaceCount["default"])
	assert.Equal(t, 1, metrics.NamespaceCount["kube-system"])

	client.shutdown()
	assert.True(t, client.stopped)
}

func TestIngressClient_EmptyStore(t *testing.T) {
	setOption := ingressSyncCheckerOption(&mockReflectorSyncChecker{})
	fakeClientSet := fake.NewSimpleClientset()
	client, err := newIngressClient(fakeClientSet, zap.NewNop(), setOption)
	assert.NoError(t, err)

	// Test with empty store
	metrics := client.GetIngressMetrics()
	assert.NotNil(t, metrics)
	assert.Empty(t, metrics.NamespaceCount)

	client.shutdown()
}

func TestTransformFuncIngress(t *testing.T) {
	info, err := transformFuncIngress(nil)
	assert.Nil(t, info)
	assert.Error(t, err)

	ingress := createTestIngress("test-ingress", "test-namespace")

	result, err := transformFuncIngress(ingress)
	assert.NoError(t, err)

	ingressInfo, ok := result.(*IngressInfo)
	assert.True(t, ok)
	assert.Equal(t, "test-ingress", ingressInfo.Name)
	assert.Equal(t, "test-namespace", ingressInfo.Namespace)
}

func TestNoOpIngressClient(t *testing.T) {
	client := &noOpIngressClient{}
	metrics := client.GetIngressMetrics()
	assert.NotNil(t, metrics)
	assert.Empty(t, metrics.NamespaceCount)

	client.shutdown()
}
