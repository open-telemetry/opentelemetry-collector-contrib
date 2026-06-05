// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapi

import (
	"context"
	"errors"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/k8snode"
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
	cfg.AuthType = k8sconfig.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

	k8sDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
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

func TestDetectClusterUIDSkipsOnForbidden(t *testing.T) {
	md := &mockMetadata{}
	md.On("NodeUID").Return("4b15c589-1a33-42cc-927a-b78ba9947095", nil)
	md.On("NodeName").Return("mainNode", nil)
	forbiddenErr := k8serrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "kube-system", errors.New("forbidden"))
	md.On("ClusterUID").Return("", forbiddenErr)
	cfg := CreateDefaultConfig()
	cfg.AuthType = k8sconfig.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

	core, logs := observer.New(zapcore.WarnLevel)
	k8sDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
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
	cfg.AuthType = k8sconfig.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

	k8sDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
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
	cfg.AuthType = k8sconfig.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

	k8sDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
	require.NoError(t, err)
	k8sDetector.(*detector).provider = md
	res, schemaURL, err := k8sDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	md.AssertExpectations(t)

	expected := map[string]any{}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}
