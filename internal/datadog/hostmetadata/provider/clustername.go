// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package provider // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/provider"

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
)

type ClusterNameProvider interface {
	ClusterName(context.Context) (string, error)
}

var _ ClusterNameProvider = (*chainClusterProvider)(nil)

type chainClusterProvider struct {
	logger       *zap.Logger
	providers    map[string]ClusterNameProvider
	priorityList []string
}

func (p *chainClusterProvider) ClusterName(ctx context.Context) (string, error) {
	for _, source := range p.priorityList {
		zapSource := zap.String("source", source)
		provider := p.providers[source]
		clusterName, err := provider.ClusterName(ctx)
		if err == nil {
			p.logger.Info("Resolved cluster name", zapSource, zap.String("cluster name", clusterName))
			return clusterName, nil
		}
		p.logger.Debug("Unavailable cluster name provider", zapSource, zap.Error(err))
	}

	return "", errors.New("no cluster name provider was available")
}

// Chain providers into a single provider that returns the first available hostname.
func ChainCluster(logger *zap.Logger, providers map[string]ClusterNameProvider, priorityList []string) (ClusterNameProvider, error) {
	for _, source := range priorityList {
		if _, ok := providers[source]; !ok {
			return nil, fmt.Errorf("%q source is not available in providers", source)
		}
	}

	return &chainClusterProvider{logger: logger, providers: providers, priorityList: priorityList}, nil
}
