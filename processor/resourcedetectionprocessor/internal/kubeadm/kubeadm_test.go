// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	clusterName = "my-cluster"
	clusterUID  = "6f1d3c1e-2b4a-4f6e-9c1d-0a1b2c3d4e5f"
)

func kubeSystem() *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system", UID: clusterUID}}
}

func kubeadmConfig(clusterConfiguration string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kubeadm-config", Namespace: "kube-system"},
		Data:       map[string]string{"ClusterConfiguration": clusterConfiguration},
	}
}

// withFakeClient makes NewDetector use a fake clientset seeded with objs.
func withFakeClient(t *testing.T, objs ...runtime.Object) {
	t.Helper()
	orig := makeClient
	makeClient = func(k8sconfig.APIConfig) (k8s.Interface, error) {
		return fake.NewClientset(objs...), nil
	}
	t.Cleanup(func() { makeClient = orig })
}

func detect(t *testing.T, cfg Config, failOnMissingMetadata bool) (map[string]any, error) {
	t.Helper()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, failOnMissingMetadata)
	require.NoError(t, err)
	res, _, err := d.Detect(t.Context())
	return res.Attributes().AsRaw(), err
}

func TestDetect(t *testing.T) {
	withFakeClient(t, kubeSystem(), kubeadmConfig("clusterName: "+clusterName))

	got, err := detect(t, CreateDefaultConfig(), true)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"k8s.cluster.name": clusterName,
		"k8s.cluster.uid":  clusterUID,
	}, got)
}

func TestDetectDisabledResourceAttributes(t *testing.T) {
	withFakeClient(t, kubeSystem(), kubeadmConfig("clusterName: "+clusterName))

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.K8sClusterName.Enabled = false

	got, err := detect(t, cfg, true)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k8s.cluster.uid": clusterUID}, got)
}

func TestDetectNotKubeadm(t *testing.T) {
	withFakeClient(t, kubeSystem())

	got, err := detect(t, CreateDefaultConfig(), false)
	require.NoError(t, err)
	assert.Empty(t, got)

	got, err = detect(t, CreateDefaultConfig(), true)
	assert.Error(t, err)
	assert.Empty(t, got)
}

func TestDetectPartial(t *testing.T) {
	tests := []struct {
		name string
		objs []runtime.Object
		want map[string]any
	}{
		{
			name: "namespace missing",
			objs: []runtime.Object{kubeadmConfig("clusterName: " + clusterName)},
			want: map[string]any{"k8s.cluster.name": clusterName},
		},
		{
			name: "malformed ClusterConfiguration",
			objs: []runtime.Object{kubeSystem(), kubeadmConfig("clusterName: [")},
			want: map[string]any{"k8s.cluster.uid": clusterUID},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFakeClient(t, tt.objs...)

			got, err := detect(t, CreateDefaultConfig(), false)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			got, err = detect(t, CreateDefaultConfig(), true)
			assert.ErrorContains(t, err, "kubeadm metadata incomplete")
			assert.Empty(t, got)
		})
	}
}

func TestNewDetectorError(t *testing.T) {
	// service account auth outside a cluster fails to build the API client
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.ErrorContains(t, err, "failed creating Kubernetes client")
	assert.Nil(t, d)
}
