// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package k8s contains the Kubernetes hostname provider
package k8s // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/internal/k8s"

import (
	"context"
	"fmt"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/provider"
)

var _ source.Provider = (*Provider)(nil)

type Provider struct {
	logger              *zap.Logger
	nodeNameProvider    nodeNameProvider
	clusterNameProvider provider.ClusterNameProvider
}

// Hostname returns the Kubernetes node name followed by the cluster name if available.
func (p *Provider) Source(ctx context.Context) (source.Source, error) {
	nodeName, err := p.nodeNameProvider.NodeName(ctx)
	if err != nil {
		return source.Source{}, fmt.Errorf("node name not available: %w", err)
	}

	clusterName, err := p.clusterNameProvider.ClusterName(ctx)
	if err != nil {
		p.logger.Debug("failed to get valid cluster name", zap.Error(err))
		return source.Source{Kind: source.HostnameKind, Identifier: nodeName}, nil
	}

	return source.Source{Kind: source.HostnameKind, Identifier: fmt.Sprintf("%s-%s", nodeName, clusterName)}, nil
}

// NewProvider creates a new Kubernetes hostname provider.
func NewProvider(logger *zap.Logger, clusterProvider provider.ClusterNameProvider) (*Provider, error) {
	return &Provider{
		logger:              logger,
		nodeNameProvider:    newNodeNameProvider(),
		clusterNameProvider: clusterProvider,
	}, nil
}
