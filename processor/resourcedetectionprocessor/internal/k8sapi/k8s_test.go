// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapi

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig/k8sconfigtypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/k8snode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8sapi/internal/metadata"
)

var _ k8snode.Provider = (*mockMetadata)(nil)

type mockMetadata struct {
	mock.Mock
}

func (m *mockMetadata) NodeUID(_ context.Context) (string, error) {
	args := m.MethodCalled("NodeUID")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) NodeName(_ context.Context) (string, error) {
	args := m.MethodCalled("NodeName")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) NodeMachineID(_ context.Context) (string, error) {
	args := m.MethodCalled("NodeMachineID")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) ClusterUID(_ context.Context) (string, error) {
	args := m.MethodCalled("ClusterUID")
	return args.String(0), args.Error(1)
}

func TestDetect(t *testing.T) {
	md := &mockMetadata{}
	md.On("NodeUID").Return("4b15c589-1a33-42cc-927a-b78ba9947095", nil)
	md.On("NodeName").Return("mainNode", nil)
	md.On("ClusterUID").Return("a7b3c1d2-e4f5-6789-abcd-ef0123456789", nil)
	cfg := CreateDefaultConfig()
	// set k8s cluster env variables and auth type to create a dummy API client
	cfg.APIConfig.AuthType = k8sconfigtypes.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

	k8sDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)
	k8sDetector.(*detector).provider = md
	res, schemaURL, err := k8sDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	md.AssertExpectations(t)

	expected := map[string]any{
		"k8s.node.name":   "mainNode",
		"k8s.node.uid":    "4b15c589-1a33-42cc-927a-b78ba9947095",
		"k8s.cluster.uid": "a7b3c1d2-e4f5-6789-abcd-ef0123456789",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestDetectHostIDDisabledByDefault(t *testing.T) {
	md := &mockMetadata{}
	md.On("NodeUID").Return("4b15c589-1a33-42cc-927a-b78ba9947095", nil)
	md.On("NodeName").Return("mainNode", nil)
	md.On("ClusterUID").Return("a7b3c1d2-e4f5-6789-abcd-ef0123456789", nil)
	cfg := CreateDefaultConfig()
	d := &detector{
		provider: md,
		logger:   zap.NewNop(),
		ra:       &cfg.ResourceAttributes,
		rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	md.AssertNotCalled(t, "NodeMachineID")
	_, ok := res.Attributes().Get("host.id")
	assert.False(t, ok, "host.id must not be detected unless enabled")
}

func TestDetectHostID(t *testing.T) {
	tt := []struct {
		name      string
		machineID string
		expected  map[string]any
	}{
		{
			name:      "machine ID reported",
			machineID: "fdbf79e8af94cb7f9e8df36789187052",
			expected: map[string]any{
				"k8s.node.name":   "mainNode",
				"k8s.node.uid":    "4b15c589-1a33-42cc-927a-b78ba9947095",
				"k8s.cluster.uid": "a7b3c1d2-e4f5-6789-abcd-ef0123456789",
				"host.id":         "fdbf79e8af94cb7f9e8df36789187052",
			},
		},
		{
			name:      "empty machine ID omits host.id",
			machineID: "",
			expected: map[string]any{
				"k8s.node.name":   "mainNode",
				"k8s.node.uid":    "4b15c589-1a33-42cc-927a-b78ba9947095",
				"k8s.cluster.uid": "a7b3c1d2-e4f5-6789-abcd-ef0123456789",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			md := &mockMetadata{}
			md.On("NodeUID").Return("4b15c589-1a33-42cc-927a-b78ba9947095", nil)
			md.On("NodeName").Return("mainNode", nil)
			md.On("NodeMachineID").Return(tc.machineID, nil)
			md.On("ClusterUID").Return("a7b3c1d2-e4f5-6789-abcd-ef0123456789", nil)
			cfg := CreateDefaultConfig()
			cfg.ResourceAttributes.HostID.Enabled = true
			d := &detector{
				provider: md,
				logger:   zap.NewNop(),
				ra:       &cfg.ResourceAttributes,
				rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
			}

			res, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			md.AssertExpectations(t)
			assert.Equal(t, tc.expected, res.Attributes().AsRaw())
		})
	}
}

func TestDetectClusterUIDSkipsOnForbidden(t *testing.T) {
	md := &mockMetadata{}
	md.On("NodeUID").Return("4b15c589-1a33-42cc-927a-b78ba9947095", nil)
	md.On("NodeName").Return("mainNode", nil)
	forbiddenErr := k8serrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "kube-system", errors.New("forbidden"))
	md.On("ClusterUID").Return("", forbiddenErr)
	cfg := CreateDefaultConfig()
	cfg.APIConfig.AuthType = k8sconfigtypes.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

	core, logs := observer.New(zapcore.WarnLevel)
	k8sDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)
	k8sDetector.(*detector).provider = md
	k8sDetector.(*detector).logger = zap.New(core)
	res, _, err := k8sDetector.Detect(t.Context())
	require.NoError(t, err)
	md.AssertExpectations(t)

	_, hasClusterUID := res.Attributes().Get("k8s.cluster.uid")
	assert.False(t, hasClusterUID, "k8s.cluster.uid should not be set when RBAC permission is missing")
	require.Equal(t, 1, logs.Len(), "expected exactly one warning log")
}

func TestDetectClusterUIDErrorsOnTransientFailure(t *testing.T) {
	md := &mockMetadata{}
	md.On("NodeUID").Return("4b15c589-1a33-42cc-927a-b78ba9947095", nil)
	md.On("NodeName").Return("mainNode", nil)
	md.On("ClusterUID").Return("", errors.New("connection refused"))
	cfg := CreateDefaultConfig()
	cfg.APIConfig.AuthType = k8sconfigtypes.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

	k8sDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)
	k8sDetector.(*detector).provider = md
	_, _, err = k8sDetector.Detect(t.Context())
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed getting k8s cluster UID")
	md.AssertExpectations(t)
}

func TestDetectDisabledResourceAttributes(t *testing.T) {
	md := &mockMetadata{}
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.K8sNodeUID.Enabled = false
	cfg.ResourceAttributes.K8sNodeName.Enabled = false
	cfg.ResourceAttributes.K8sClusterUID.Enabled = false
	// set k8s cluster env variables and auth type to create a dummy API client
	cfg.APIConfig.AuthType = k8sconfigtypes.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

	k8sDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)
	k8sDetector.(*detector).provider = md
	res, schemaURL, err := k8sDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	md.AssertExpectations(t)

	expected := map[string]any{}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestDetectNodeErrors(t *testing.T) {
	someErr := errors.New("node not found")
	tt := []struct {
		name                  string
		uidErr                error
		nameErr               error
		machineIDErr          error
		failOnMissingMetadata bool
		wantErr               string
	}{
		{name: "node uid error ignored", uidErr: someErr},
		{name: "node uid error returned", uidErr: someErr, failOnMissingMetadata: true, wantErr: "failed getting k8s node UID"},
		{name: "node name error ignored", nameErr: someErr},
		{name: "node name error returned", nameErr: someErr, failOnMissingMetadata: true, wantErr: "failed getting k8s node name"},
		{name: "node machine ID error ignored", machineIDErr: someErr},
		{name: "node machine ID error returned", machineIDErr: someErr, failOnMissingMetadata: true, wantErr: "failed getting k8s node machine ID"},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			md := &mockMetadata{}
			md.On("NodeUID").Return("4b15c589-1a33-42cc-927a-b78ba9947095", tc.uidErr)
			md.On("NodeName").Return("mainNode", tc.nameErr).Maybe()
			md.On("NodeMachineID").Return("fdbf79e8af94cb7f9e8df36789187052", tc.machineIDErr).Maybe()
			cfg := CreateDefaultConfig()
			cfg.ResourceAttributes.HostID.Enabled = true
			d := &detector{
				provider:              md,
				logger:                zap.NewNop(),
				ra:                    &cfg.ResourceAttributes,
				rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
				failOnMissingMetadata: tc.failOnMissingMetadata,
			}

			res, schemaURL, err := d.Detect(t.Context())
			if tc.wantErr != "" {
				require.ErrorIs(t, err, someErr)
				assert.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Empty(t, schemaURL)
			assert.Equal(t, 0, res.Attributes().Len())
			md.AssertExpectations(t)
		})
	}
}

// stubProvider makes the detector create its k8snode provider with fn.
func stubProvider(t *testing.T, fn func(string, k8sconfig.APIConfig) (k8snode.Provider, error)) {
	t.Helper()
	orig := newProvider
	newProvider = fn
	t.Cleanup(func() { newProvider = orig })
}

func newTestDetector(t *testing.T, cfg Config, failOnMissingMetadata bool) internal.Detector {
	t.Helper()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, failOnMissingMetadata)
	require.NoError(t, err)
	require.NotNil(t, d)
	return d
}

func TestNewDetectorOutsideKubernetes(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		nodeName string
	}{
		{name: "missing service env and node name"},
		{name: "missing service env", nodeName: "mainNode"},
		{name: "missing service port", host: "10.96.0.1", nodeName: "mainNode"},
		{name: "missing service host", port: "443", nodeName: "mainNode"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KUBERNETES_SERVICE_HOST", tc.host)
			t.Setenv("KUBERNETES_SERVICE_PORT", tc.port)
			t.Setenv("K8S_NODE_NAME", tc.nodeName)
			stubProvider(t, func(string, k8sconfig.APIConfig) (k8snode.Provider, error) {
				t.Fatal("the k8snode provider must not be created outside Kubernetes")
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
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "")
	stubProvider(t, func(string, k8sconfig.APIConfig) (k8snode.Provider, error) {
		t.Fatal("the k8snode provider must not be created without a node name")
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
// the configured node_from_env_var and API settings reach the provider.
func TestDetectExternalKubeconfig(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	t.Setenv("MY_NODE", "mainNode")

	md := &mockMetadata{}
	md.On("NodeUID").Return("4b15c589-1a33-42cc-927a-b78ba9947095", nil)
	md.On("NodeName").Return("mainNode", nil)
	md.On("ClusterUID").Return("a7b3c1d2-e4f5-6789-abcd-ef0123456789", nil)
	stubProvider(t, func(nodeName string, apiConf k8sconfig.APIConfig) (k8snode.Provider, error) {
		assert.Equal(t, "mainNode", nodeName)
		assert.Equal(t, k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig, Context: "remote", KubeAPIQPS: 12.5, KubeAPIBurst: 25}, apiConf)
		return md, nil
	})

	cfg := CreateDefaultConfig()
	cfg.NodeFromEnvVar = "MY_NODE"
	cfg.APIConfig = k8sconfigtypes.APIConfig{AuthType: k8sconfigtypes.AuthTypeKubeConfig, Context: "remote", KubeAPIQPS: 12.5, KubeAPIBurst: 25}

	res, _, err := newTestDetector(t, cfg, true).Detect(t.Context())
	require.NoError(t, err)
	md.AssertExpectations(t)
	assert.Equal(t, map[string]any{
		"k8s.node.name":   "mainNode",
		"k8s.node.uid":    "4b15c589-1a33-42cc-927a-b78ba9947095",
		"k8s.cluster.uid": "a7b3c1d2-e4f5-6789-abcd-ef0123456789",
	}, res.Attributes().AsRaw())
}

// TestDetectKubeconfigError checks that a client that cannot be built from an explicit
// configuration is reported, not mistaken for running outside Kubernetes.
func TestDetectKubeconfigError(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	t.Setenv("K8S_NODE_NAME", "mainNode")
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

// TestDetectRetriesProviderCreation checks that a failed provider creation is not cached.
func TestDetectRetriesProviderCreation(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

	md := &mockMetadata{}
	md.On("NodeUID").Return("4b15c589-1a33-42cc-927a-b78ba9947095", nil)
	md.On("NodeName").Return("mainNode", nil)
	md.On("ClusterUID").Return("a7b3c1d2-e4f5-6789-abcd-ef0123456789", nil)
	transientErr := errors.New("failed to read service account token")
	calls := 0
	stubProvider(t, func(string, k8sconfig.APIConfig) (k8snode.Provider, error) {
		calls++
		if calls == 1 {
			return nil, transientErr
		}
		return md, nil
	})

	d := newTestDetector(t, CreateDefaultConfig(), false)
	assert.Zero(t, calls, "NewDetector must not create the provider")

	_, _, err := d.Detect(t.Context())
	require.ErrorIs(t, err, transientErr)
	assert.ErrorContains(t, err, "failed creating k8snode detector")

	for range 2 {
		res, _, err := d.Detect(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 3, res.Attributes().Len())
	}
	assert.Equal(t, 2, calls, "the provider must be created once after a failed attempt")
}

func TestDetectConcurrent(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

	md := &mockMetadata{}
	md.On("NodeUID").Return("4b15c589-1a33-42cc-927a-b78ba9947095", nil)
	md.On("NodeName").Return("mainNode", nil)
	md.On("ClusterUID").Return("a7b3c1d2-e4f5-6789-abcd-ef0123456789", nil)
	var calls atomic.Int32
	stubProvider(t, func(string, k8sconfig.APIConfig) (k8snode.Provider, error) {
		calls.Add(1)
		return md, nil
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
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

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
