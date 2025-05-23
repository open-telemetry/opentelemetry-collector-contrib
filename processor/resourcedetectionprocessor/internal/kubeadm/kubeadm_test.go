// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/kubeadm"
)

var _ kubeadm.Provider = (*mockMetadata)(nil)

type mockMetadata struct {
	mock.Mock
}

func (m *mockMetadata) ClusterName(_ context.Context) (string, error) {
	args := m.MethodCalled("ClusterName")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) ClusterUID(_ context.Context) (string, error) {
	args := m.MethodCalled("ClusterUID")
	return args.String(0), args.Error(1)
}

func TestDetect(t *testing.T) {
	md := &mockMetadata{}
	md.On("ClusterName").Return("cluster-1", nil)
	md.On("ClusterUID").Return("uid-1", nil)
	cfg := CreateDefaultConfig()
	// set k8s cluster env variables and auth type to create a dummy API client
	cfg.AuthType = k8sconfig.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	k8sDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
	require.NoError(t, err)
	k8sDetector.(*detector).provider = md
	res, schemaURL, err := k8sDetector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	md.AssertExpectations(t)

	expected := map[string]any{
		string(conventions.K8SClusterNameKey): "cluster-1",
		"k8s.cluster.uid":                     "uid-1",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestDetectDisabledResourceAttributes(t *testing.T) {
	md := &mockMetadata{}
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.K8sClusterName.Enabled = false
	cfg.ResourceAttributes.K8sClusterUID.Enabled = false
	// set k8s cluster env variables and auth type to create a dummy API client
	cfg.AuthType = k8sconfig.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	k8sDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
	require.NoError(t, err)
	k8sDetector.(*detector).provider = md
	res, schemaURL, err := k8sDetector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	md.AssertExpectations(t)

	expected := map[string]any{}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}
