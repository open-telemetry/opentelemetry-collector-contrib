// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/provider"
)

var _ provider.ClusterNameProvider = (*StringProvider)(nil)
var _ nodeNameProvider = (*StringProvider)(nil)

type StringProvider string

func (p StringProvider) ClusterName(context.Context) (string, error) { return string(p), nil }
func (p StringProvider) NodeName(context.Context) (string, error)    { return string(p), nil }

var _ provider.ClusterNameProvider = (*ErrorProvider)(nil)
var _ nodeNameProvider = (*ErrorProvider)(nil)

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

			src, err := provider.Source(context.Background())
			if err != nil || testInstance.err != "" {
				assert.EqualError(t, err, testInstance.err)
			} else {
				assert.Equal(t, testInstance.src, src)
			}
		})
	}
}
