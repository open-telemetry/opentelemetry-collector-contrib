// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/provider"
)

var (
	_ provider.ClusterNameProvider = (*StringProvider)(nil)
	_ nodeNameProvider             = (*StringProvider)(nil)
)

type StringProvider string

func (p StringProvider) ClusterName(context.Context) (string, error) { return string(p), nil }
func (p StringProvider) NodeName(context.Context) (string, error)    { return string(p), nil }

var (
	_ provider.ClusterNameProvider = (*ErrorProvider)(nil)
	_ nodeNameProvider             = (*ErrorProvider)(nil)
)

type ErrorProvider string

func (p ErrorProvider) ClusterName(context.Context) (string, error) { return "", errors.New(string(p)) }
func (p ErrorProvider) NodeName(context.Context) (string, error)    { return "", errors.New(string(p)) }

func TestProvider(t *testing.T) {
	tests := []struct {
		name string

		nodeNameProvider    nodeNameProvider
		clusterNameProvider provider.ClusterNameProvider

		src source.Source
		err string
	}{
		{
			name:                "no node name",
			nodeNameProvider:    ErrorProvider("errNodeName"),
			clusterNameProvider: StringProvider("clusterName"),
			err:                 "node name not available: errNodeName",
		},
		{
			name:                "node name but no cluster name",
			nodeNameProvider:    StringProvider("nodeName"),
			clusterNameProvider: ErrorProvider("errClusterName"),
			src:                 source.Source{Kind: source.HostnameKind, Identifier: "nodeName"},
		},
		{
			name:                "node and cluster name",
			nodeNameProvider:    StringProvider("nodeName"),
			clusterNameProvider: StringProvider("clusterName"),
			src:                 source.Source{Kind: source.HostnameKind, Identifier: "nodeName-clusterName"},
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			provider := &Provider{
				logger:              zap.NewNop(),
				nodeNameProvider:    testInstance.nodeNameProvider,
				clusterNameProvider: testInstance.clusterNameProvider,
			}

			src, err := provider.Source(t.Context())
			if err != nil || testInstance.err != "" {
				assert.EqualError(t, err, testInstance.err)
			} else {
				assert.Equal(t, testInstance.src, src)
			}
		})
	}
}

func TestIsAvailable(t *testing.T) {
	t.Run("unavailable when node name provider fails at construction", func(t *testing.T) {
		p := &Provider{
			logger:              zap.NewNop(),
			nodeNameProvider:    &nodeNameUnavailable{err: errors.New("no k8s")},
			clusterNameProvider: StringProvider("cluster"),
		}
		assert.False(t, p.IsAvailable())
	})

	t.Run("available when node name provider is functional", func(t *testing.T) {
		p := &Provider{
			logger:              zap.NewNop(),
			nodeNameProvider:    StringProvider("node"),
			clusterNameProvider: StringProvider("cluster"),
		}
		assert.True(t, p.IsAvailable())
	})
}

func TestHostAlias(t *testing.T) {
	tests := []struct {
		name string

		nodeNameProvider    nodeNameProvider
		clusterNameProvider provider.ClusterNameProvider

		expectedAlias string
		expectedErr   string
	}{
		{
			name:                "node name unavailable returns error",
			nodeNameProvider:    ErrorProvider("errNodeName"),
			clusterNameProvider: StringProvider("clusterName"),
			expectedErr:         "node name not available: errNodeName",
		},
		{
			name:                "cluster name unavailable returns error",
			nodeNameProvider:    StringProvider("nodeName"),
			clusterNameProvider: ErrorProvider("errClusterName"),
			expectedErr:         "cluster name not available: errClusterName",
		},
		{
			name:                "node and cluster name",
			nodeNameProvider:    StringProvider("nodeName"),
			clusterNameProvider: StringProvider("clusterName"),
			expectedAlias:       "nodeName-clusterName",
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			p := &Provider{
				logger:              zap.NewNop(),
				nodeNameProvider:    testInstance.nodeNameProvider,
				clusterNameProvider: testInstance.clusterNameProvider,
			}

			alias, err := p.HostAlias(t.Context())
			if testInstance.expectedErr != "" {
				assert.EqualError(t, err, testInstance.expectedErr)
				assert.Empty(t, alias)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testInstance.expectedAlias, alias)
			}
		})
	}
}
