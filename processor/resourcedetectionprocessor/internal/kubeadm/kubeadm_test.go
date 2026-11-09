// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"errors"
	"sync"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig/k8sconfigtypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
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

// withFakeClient makes the detector use a fake clientset seeded with objs, as if it ran
// in a Kubernetes pod.
func withFakeClient(t *testing.T, objs ...runtime.Object) {
	t.Helper()
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
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

func TestNewDetectorOutsideKubernetes(t *testing.T) {
	tests := []struct {
		name string
		host string
		port string
	}{
		{name: "missing service env"},
		{name: "missing service port", host: "10.96.0.1"},
		{name: "missing service host", port: "443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KUBERNETES_SERVICE_HOST", tt.host)
			t.Setenv("KUBERNETES_SERVICE_PORT", tt.port)
			stubMakeClient(t, func(k8sconfig.APIConfig) (k8s.Interface, error) {
				t.Fatal("the Kubernetes client must not be created outside Kubernetes")
				return nil, nil
			})

			got, err := detect(t, CreateDefaultConfig(), false)
			require.NoError(t, err)
			assert.Empty(t, got)

			got, err = detect(t, CreateDefaultConfig(), true)
			require.ErrorIs(t, err, internal.ErrNotInKubernetes)
			assert.Empty(t, got)
		})
	}
}

func TestDetectConcurrent(t *testing.T) {
	calls := 0
	stubMakeClient(t, func(k8sconfig.APIConfig) (k8s.Interface, error) {
		calls++ // guarded by the detector's mutex
		return fake.NewClientset(kubeSystem(), kubeadmConfig("clusterName: "+clusterName)), nil
	})
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			res, _, err := d.Detect(t.Context())
			assert.NoError(t, err)
			assert.Equal(t, 2, res.Attributes().Len())
		})
	}
	wg.Wait()
	assert.Equal(t, 1, calls)
}

func TestNewDetectorInvalidAPIConfig(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.APIConfig.AuthType = "invalid"

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.ErrorContains(t, err, "invalid authType for kubernetes")
	assert.Nil(t, d)
}

// TestDetectPreservesAPIConfig checks that an explicit kubeconfig works outside a pod and
// that the configured API settings reach the client.
func TestDetectPreservesAPIConfig(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	calls := 0
	stubMakeClient(t, func(cfg k8sconfig.APIConfig) (k8s.Interface, error) {
		calls++
		assert.Equal(t, k8sconfig.APIConfig{
			AuthType:     k8sconfig.AuthTypeKubeConfig,
			Context:      "custom-context",
			KubeAPIQPS:   12.5,
			KubeAPIBurst: 25,
		}, cfg)
		return fake.NewClientset(kubeSystem(), kubeadmConfig("clusterName: "+clusterName)), nil
	})

	cfg := CreateDefaultConfig()
	cfg.APIConfig = k8sconfigtypes.APIConfig{
		AuthType:     k8sconfigtypes.AuthTypeKubeConfig,
		Context:      "custom-context",
		KubeAPIQPS:   12.5,
		KubeAPIBurst: 25,
	}
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, true)
	require.NoError(t, err)
	assert.Zero(t, calls, "NewDetector must not create the Kubernetes client")

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"k8s.cluster.name": clusterName,
		"k8s.cluster.uid":  clusterUID,
	}, res.Attributes().AsRaw())
	assert.Equal(t, 1, calls)
}

// TestDetectRetriesClientCreation checks that a failure to create the client is reported
// whatever fail_on_missing_metadata says, and is not cached.
func TestDetectRetriesClientCreation(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	clientErr := errors.New("failed to read service account token")

	for _, failOnMissingMetadata := range []bool{false, true} {
		calls := 0
		stubMakeClient(t, func(k8sconfig.APIConfig) (k8s.Interface, error) {
			calls++
			if calls == 1 {
				return nil, clientErr
			}
			return fake.NewClientset(kubeSystem(), kubeadmConfig("clusterName: "+clusterName)), nil
		})

		d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), failOnMissingMetadata)
		require.NoError(t, err)

		_, _, err = d.Detect(t.Context())
		require.ErrorIs(t, err, clientErr)
		assert.ErrorContains(t, err, "failed creating Kubernetes client")

		for range 2 {
			res, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			assert.Equal(t, 2, res.Attributes().Len())
		}
		assert.Equal(t, 2, calls, "the client must be created once after a failed attempt")
	}
}

// stubMakeClient makes the detector create its Kubernetes client with fn.
func stubMakeClient(t *testing.T, fn func(k8sconfig.APIConfig) (k8s.Interface, error)) {
	t.Helper()
	orig := makeClient
	makeClient = fn
	t.Cleanup(func() { makeClient = orig })
}
