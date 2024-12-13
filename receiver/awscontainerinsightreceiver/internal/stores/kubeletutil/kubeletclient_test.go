// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	kube "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
)

type mockClientProvider struct {
	endpoint string
	cfg      *kube.ClientConfig
}

func (cp *mockClientProvider) BuildClient() (kube.Client, error) {
	return &fakeClient{
		endpoint: cp.endpoint,
	}, nil
}

type fakeClient struct {
	endpoint string
}

func (f *fakeClient) Get(path string) ([]byte, error) {
	return []byte(path), nil
}

func TestNewKubeletClient(t *testing.T) {
	kubeletNewClientProvider = func(endpoint string, cfg *kube.ClientConfig, _ *zap.Logger) (kube.ClientProvider, error) {
		return &mockClientProvider{
			endpoint: endpoint,
			cfg:      cfg,
		}, nil
	}

	tests := []struct {
		kubeIP       string
		port         string
		wantEndpoint string
	}{
		{
			kubeIP:       "0.0.0.0",
			port:         "8000",
			wantEndpoint: "0.0.0.0:8000",
		},
		{
			kubeIP:       "2001:db8:3333:4444:5555:6666:7777:8888",
			port:         "8000",
			wantEndpoint: "[2001:db8:3333:4444:5555:6666:7777:8888]:8000",
		},
		{
			kubeIP:       "2001:db8:3333:4444:5555:6666:1.2.3.4",
			port:         "8000",
			wantEndpoint: "[2001:db8:3333:4444:5555:6666:1.2.3.4]:8000",
		},
	}
	for _, tt := range tests {
		client, err := NewKubeletClient(tt.kubeIP, tt.port, nil, zap.NewNop())
		require.NoError(t, err)
		assert.Equal(t, client.KubeIP, tt.kubeIP)
		fc := (client.restClient).(*fakeClient)
		assert.NotNil(t, fc)
		assert.Equal(t, tt.wantEndpoint, fc.endpoint)
	}
}
