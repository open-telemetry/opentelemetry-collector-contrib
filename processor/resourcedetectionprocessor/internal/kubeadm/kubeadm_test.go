// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/kubeadm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm/internal/metadata"
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
	cfg.APIConfig.AuthType = k8sconfig.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	k8sDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)
	k8sDetector.(*detector).provider = md
	res, schemaURL, err := k8sDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	md.AssertExpectations(t)

	expected := map[string]any{
		"k8s.cluster.name": "cluster-1",
		"k8s.cluster.uid":  "uid-1",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestDetectDisabledResourceAttributes(t *testing.T) {
	md := &mockMetadata{}
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.K8sClusterName.Enabled = false
	cfg.ResourceAttributes.K8sClusterUID.Enabled = false
	// set k8s cluster env variables and auth type to create a dummy API client
	cfg.APIConfig.AuthType = k8sconfig.AuthTypeNone
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

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

func TestDetectErrors(t *testing.T) {
	someErr := errors.New("configmap not found")
	tt := []struct {
		name                  string
		nameErr               error
		uidErr                error
		failOnMissingMetadata bool
		wantErr               string
	}{
		{name: "cluster name error ignored", nameErr: someErr},
		{name: "cluster name error returned", nameErr: someErr, failOnMissingMetadata: true, wantErr: "failed getting k8s cluster name"},
		{name: "cluster uid error ignored", uidErr: someErr},
		{name: "cluster uid error returned", uidErr: someErr, failOnMissingMetadata: true, wantErr: "failed getting k8s cluster uid"},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			md := &mockMetadata{}
			md.On("ClusterName").Return("cluster-1", tc.nameErr)
			md.On("ClusterUID").Return("uid-1", tc.uidErr).Maybe()
			cfg := CreateDefaultConfig()
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

func TestNewDetectorError(t *testing.T) {
	// service account auth outside a cluster fails to build the API client
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.ErrorContains(t, err, "failed creating kubeadm provider")
	assert.Nil(t, d)
}
