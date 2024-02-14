// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8snode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

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

func TestDetect(t *testing.T) {
	md := &mockMetadata{}
	md.On("NodeUID").Return("4b15c589-1a33-42cc-927a-b78ba9947095", nil)
	md.On("NodeName").Return("mainNode", nil)
	cfg := CreateDefaultConfig()
	// set k8s cluster env variables and auth type to create a dummy API client
	cfg.APIConfig.AuthType = k8sconfig.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	t.Setenv("K8S_NODE_NAME", "mainNode")

	k8sDetector, err := NewDetector(processortest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)
	k8sDetector.(*detector).provider = md
	res, schemaURL, err := k8sDetector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	md.AssertExpectations(t)

	expected := map[string]any{
		conventions.AttributeK8SNodeName: "mainNode",
		conventions.AttributeK8SNodeUID:  "4b15c589-1a33-42cc-927a-b78ba9947095",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}
