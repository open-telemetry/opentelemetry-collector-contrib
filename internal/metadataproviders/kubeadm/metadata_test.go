// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestNewProvider(t *testing.T) {
	// set k8s cluster env variables to make the API client happy
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	_, err := NewProvider("name", "ns", k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeNone})
	assert.NoError(t, err)
}

func TestClusterName(t *testing.T) {
	client := fake.NewSimpleClientset()
	err := setupConfigMap(client)
	assert.NoError(t, err)

	tests := []struct {
		testName    string
		CMname      string
		CMnamespace string
		clusterName string
		errMsg      string
	}{
		{
			testName:    "valid",
			CMname:      "cm",
			CMnamespace: "ns",
			clusterName: "myClusterName",
			errMsg:      "",
		},
		{
			testName:    "configmap not found",
			CMname:      "cm2",
			CMnamespace: "ns",
			errMsg:      "failed to fetch ConfigMap with name cm2 and namespace ns from K8s API: configmaps \"cm2\" not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			kubeadmP := &kubeadmProvider{
				kubeadmClient:      client,
				configMapName:      tt.CMname,
				configMapNamespace: tt.CMnamespace,
			}
			clusterName, err := kubeadmP.ClusterName(context.Background())
			if tt.errMsg != "" {
				assert.EqualError(t, err, tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, clusterName, tt.clusterName)
			}
		})
	}
}

func setupConfigMap(client *fake.Clientset) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm",
			Namespace: "ns",
		},
		Data: map[string]string{
			"clusterName": "myClusterName",
		},
	}
	_, err := client.CoreV1().ConfigMaps("ns").Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}
