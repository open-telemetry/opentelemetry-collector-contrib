// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_k8s_api) && !omit_detector_k8s_api

package k8sapi

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig/k8sconfigtypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	nodeName   = "mainNode"
	nodeUID    = "4b15c589-1a33-42cc-927a-b78ba9947095"
	clusterUID = "a7b3c1d2-e4f5-6789-abcd-ef0123456789"
	machineID  = "fdbf79e8af94cb7f9e8df36789187052"
)

var defaultAttributes = map[string]any{
	"k8s.node.name":   nodeName,
	"k8s.node.uid":    nodeUID,
	"k8s.cluster.uid": clusterUID,
}

// newFakeClient returns a clientset serving the node, with the given machine ID, and the
// kube-system namespace.
func newFakeClient(machineID string) *fake.Clientset {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName, UID: nodeUID}}
	node.Status.NodeInfo.MachineID = machineID
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system", UID: clusterUID}}
	return fake.NewClientset(node, ns)
}

// failGet makes every get of resource on client fail with err.
func failGet(client *fake.Clientset, resource string, err error) {
	client.PrependReactor("get", resource, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
}

// stubClient makes the detector create its Kubernetes client with fn.
func stubClient(t *testing.T, fn func(k8sconfig.APIConfig) (kubernetes.Interface, error)) {
	t.Helper()
	orig := newClient
	newClient = fn
	t.Cleanup(func() { newClient = orig })
}

// useClient makes the detector use client.
func useClient(t *testing.T, client kubernetes.Interface) {
	t.Helper()
	stubClient(t, func(k8sconfig.APIConfig) (kubernetes.Interface, error) { return client, nil })
}

// inCluster sets the environment of a pod running on nodeName.
func inCluster(t *testing.T) {
	t.Helper()
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", nodeName)
}

func newTestDetector(t *testing.T, cfg Config, failOnMissingMetadata bool) internal.Detector {
	t.Helper()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, failOnMissingMetadata)
	require.NoError(t, err)
	require.NotNil(t, d)
	return d
}

func TestDetect(t *testing.T) {
	inCluster(t)
	useClient(t, newFakeClient(machineID))

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	assert.Equal(t, defaultAttributes, res.Attributes().AsRaw(), "host.id must not be detected unless enabled")
}

func TestDetectHostID(t *testing.T) {
	tt := []struct {
		name      string
		machineID string
		expected  map[string]any
	}{
		{
			name:      "machine ID reported",
			machineID: machineID,
			expected: map[string]any{
				"k8s.node.name":   nodeName,
				"k8s.node.uid":    nodeUID,
				"k8s.cluster.uid": clusterUID,
				"host.id":         machineID,
			},
		},
		{
			name:      "empty machine ID omits host.id",
			machineID: "",
			expected:  defaultAttributes,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			inCluster(t)
			useClient(t, newFakeClient(tc.machineID))
			cfg := CreateDefaultConfig()
			cfg.ResourceAttributes.HostID.Enabled = true

			res, _, err := newTestDetector(t, cfg, false).Detect(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tc.expected, res.Attributes().AsRaw())
		})
	}
}

func TestDetectClusterUIDSkipsOnForbidden(t *testing.T) {
	for _, clusterErr := range []error{
		k8serrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "kube-system", errors.New("forbidden")),
		k8serrors.NewUnauthorized("unauthorized"),
	} {
		t.Run(string(k8serrors.ReasonForError(clusterErr)), func(t *testing.T) {
			inCluster(t)
			client := newFakeClient("")
			failGet(client, "namespaces", clusterErr)
			useClient(t, client)

			core, logs := observer.New(zapcore.WarnLevel)
			d := newTestDetector(t, CreateDefaultConfig(), false)
			d.(*detector).logger = zap.New(core)
			res, _, err := d.Detect(t.Context())
			require.NoError(t, err)

			assert.Equal(t, map[string]any{"k8s.node.name": nodeName, "k8s.node.uid": nodeUID}, res.Attributes().AsRaw(),
				"k8s.cluster.uid should not be set when RBAC permission is missing")
			require.Equal(t, 1, logs.Len(), "expected exactly one warning log")
		})
	}
}

func TestDetectClusterUIDErrorsOnTransientFailure(t *testing.T) {
	inCluster(t)
	client := newFakeClient("")
	failGet(client, "namespaces", errors.New("connection refused"))
	useClient(t, client)

	_, _, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	assert.ErrorContains(t, err, "failed getting k8s cluster UID")
	assert.ErrorContains(t, err, "connection refused")
}

func TestDetectClusterUIDErrorIgnoredWhenDisabled(t *testing.T) {
	inCluster(t)
	client := newFakeClient("")
	failGet(client, "namespaces", errors.New("connection refused"))
	useClient(t, client)
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.K8sClusterUID.Enabled = false

	res, _, err := newTestDetector(t, cfg, true).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k8s.node.name": nodeName, "k8s.node.uid": nodeUID}, res.Attributes().AsRaw())
}

func TestDetectDisabledResourceAttributes(t *testing.T) {
	inCluster(t)
	useClient(t, newFakeClient(machineID))
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.K8sNodeUID.Enabled = false
	cfg.ResourceAttributes.K8sNodeName.Enabled = false
	cfg.ResourceAttributes.K8sClusterUID.Enabled = false

	res, schemaURL, err := newTestDetector(t, cfg, false).Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	assert.Equal(t, map[string]any{}, res.Attributes().AsRaw())
}

func TestDetectNodeErrors(t *testing.T) {
	nodeErr := errors.New("node not found")
	for _, failOnMissingMetadata := range []bool{false, true} {
		inCluster(t)
		client := newFakeClient(machineID)
		failGet(client, "nodes", nodeErr)
		useClient(t, client)

		res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), failOnMissingMetadata).Detect(t.Context())
		if failOnMissingMetadata {
			require.ErrorIs(t, err, nodeErr)
			assert.ErrorContains(t, err, "failed getting k8s node")
		} else {
			require.NoError(t, err)
		}
		assert.Empty(t, schemaURL)
		assert.Equal(t, 0, res.Attributes().Len())
	}
}

func TestNewDetectorOutsideKubernetes(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		nodeName string
	}{
		{name: "missing service env and node name"},
		{name: "missing service env", nodeName: nodeName},
		{name: "missing service port", host: "10.96.0.1", nodeName: nodeName},
		{name: "missing service host", port: "443", nodeName: nodeName},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KUBERNETES_SERVICE_HOST", tc.host)
			t.Setenv("KUBERNETES_SERVICE_PORT", tc.port)
			t.Setenv("K8S_NODE_NAME", tc.nodeName)
			stubClient(t, func(k8sconfig.APIConfig) (kubernetes.Interface, error) {
				t.Fatal("the Kubernetes client must not be created outside Kubernetes")
				return nil, nil
			})

			res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
			require.NoError(t, err)
			assert.Empty(t, schemaURL)
			assert.Equal(t, 0, res.Attributes().Len())

			res, _, err = newTestDetector(t, CreateDefaultConfig(), true).Detect(t.Context())
			require.ErrorIs(t, err, internal.ErrNotInKubernetes)
			assert.ErrorContains(t, err, "k8s node metadata unavailable: not running in Kubernetes")
			assert.Equal(t, 0, res.Attributes().Len())
		})
	}
}

func TestDetectMissingNodeName(t *testing.T) {
	inCluster(t)
	t.Setenv("K8S_NODE_NAME", "")
	stubClient(t, func(k8sconfig.APIConfig) (kubernetes.Interface, error) {
		t.Fatal("the Kubernetes client must not be created without a node name")
		return nil, nil
	})

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	require.NoError(t, err)
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())

	_, _, err = newTestDetector(t, CreateDefaultConfig(), true).Detect(t.Context())
	assert.EqualError(t, err, "k8s node metadata unavailable: node name can't be found. Check the readme on how to set the required env variable")
}

func TestNewDetectorInvalidAPIConfig(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.APIConfig.KubeAPIQPS = -1

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.ErrorContains(t, err, "kube_api_qps must be greater than 0")
	assert.Nil(t, d)
}

// TestDetectExternalKubeconfig checks that kubeConfig auth works outside a pod and that
// the configured node_from_env_var and API settings reach the client.
func TestDetectExternalKubeconfig(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	t.Setenv("MY_NODE", nodeName)

	client := newFakeClient("")
	stubClient(t, func(apiConf k8sconfig.APIConfig) (kubernetes.Interface, error) {
		assert.Equal(t, k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig, Context: "remote", KubeAPIQPS: 12.5, KubeAPIBurst: 25}, apiConf)
		return client, nil
	})

	cfg := CreateDefaultConfig()
	cfg.NodeFromEnvVar = "MY_NODE"
	cfg.APIConfig = k8sconfigtypes.APIConfig{AuthType: k8sconfigtypes.AuthTypeKubeConfig, Context: "remote", KubeAPIQPS: 12.5, KubeAPIBurst: 25}

	res, _, err := newTestDetector(t, cfg, true).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, defaultAttributes, res.Attributes().AsRaw())
}

// TestDetectKubeconfigError checks that a client that cannot be built from an explicit
// configuration is reported, not mistaken for running outside Kubernetes.
func TestDetectKubeconfigError(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	t.Setenv("K8S_NODE_NAME", nodeName)
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("HOME", t.TempDir())

	cfg := CreateDefaultConfig()
	cfg.APIConfig.AuthType = k8sconfigtypes.AuthTypeKubeConfig

	for _, failOnMissingMetadata := range []bool{false, true} {
		_, _, err := newTestDetector(t, cfg, failOnMissingMetadata).Detect(t.Context())
		require.ErrorContains(t, err, "failed creating k8snode detector")
		assert.NotErrorIs(t, err, internal.ErrNotInKubernetes)
	}
}

// TestDetectRetriesClientCreation checks that a failed client creation is not cached.
func TestDetectRetriesClientCreation(t *testing.T) {
	inCluster(t)
	client := newFakeClient("")
	transientErr := errors.New("failed to read service account token")
	calls := 0
	stubClient(t, func(k8sconfig.APIConfig) (kubernetes.Interface, error) {
		calls++
		if calls == 1 {
			return nil, transientErr
		}
		return client, nil
	})

	d := newTestDetector(t, CreateDefaultConfig(), false)
	assert.Zero(t, calls, "NewDetector must not create the client")

	_, _, err := d.Detect(t.Context())
	require.ErrorIs(t, err, transientErr)
	assert.ErrorContains(t, err, "failed creating k8snode detector")

	for range 2 {
		res, _, err := d.Detect(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 3, res.Attributes().Len())
	}
	assert.Equal(t, 2, calls, "the client must be created once after a failed attempt")
}

func TestDetectConcurrent(t *testing.T) {
	inCluster(t)
	client := newFakeClient("")
	var calls atomic.Int32
	stubClient(t, func(k8sconfig.APIConfig) (kubernetes.Interface, error) {
		calls.Add(1)
		return client, nil
	})

	d := newTestDetector(t, CreateDefaultConfig(), false)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, _, err := d.Detect(t.Context())
			assert.NoError(t, err)
		})
	}
	wg.Wait()
	assert.Equal(t, int32(1), calls.Load())
}

func TestNewDeprecatedDetector(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.APIConfig.AuthType = k8sconfigtypes.AuthTypeNone
	inCluster(t)

	core, logs := observer.New(zapcore.WarnLevel)
	set := processortest.NewNopSettings(processortest.NopType)
	set.Logger = zap.New(core)
	d, err := NewDeprecatedDetector(set, cfg, false)
	require.NoError(t, err)
	assert.NotNil(t, d)
	require.Equal(t, 1, logs.FilterMessageSnippet("k8snode detector name is deprecated").Len())
}

func TestNewDeprecatedDetectorOutsideKubernetes(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	t.Setenv("K8S_NODE_NAME", "")

	d, err := NewDeprecatedDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 0, res.Attributes().Len())
}
