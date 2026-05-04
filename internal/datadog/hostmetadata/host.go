// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata"

import (
	"fmt"
	"time"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/ecs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/gcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/k8s"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/system"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/provider"
)

// GetSourceAndAliasProviders builds the hostname resolution chain and any host alias providers.
func GetSourceAndAliasProviders(set component.TelemetrySettings, configHostname string, timeout time.Duration) (source.Provider, []HostAliasProvider, error) {
	ecsProvider, err := ecs.NewProvider(set)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build ECS Fargate provider: %w", err)
	}

	azureProvider := azure.NewProvider()
	ec2Provider, err := ec2.NewProvider(set.Logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build EC2 provider: %w", err)
	}
	gcpProvider := gcp.NewProvider()

	clusterNameProvider, err := provider.ChainCluster(set.Logger,
		map[string]provider.ClusterNameProvider{
			"azure": azureProvider,
			"ec2":   ec2Provider,
			"gcp":   gcpProvider,
		},
		[]string{"azure", "ec2", "gcp"},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build Kubernetes cluster name provider: %w", err)
	}

	k8sProvider, err := k8s.NewProvider(set.Logger, clusterNameProvider)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build Kubernetes hostname provider: %w", err)
	}

	chain, err := provider.Chain(
		set.Logger,
		map[string]source.Provider{
			"config":     provider.Config(configHostname),
			"azure":      azureProvider,
			"ecs":        ecsProvider,
			"ec2":        ec2Provider,
			"gcp":        gcpProvider,
			"kubernetes": k8sProvider,
			"system":     system.NewProvider(set.Logger),
		},
		[]string{"config", "azure", "ecs", "ec2", "gcp", "kubernetes", "system"},
		timeout,
	)
	if err != nil {
		return nil, nil, err
	}

	var aliases []HostAliasProvider
	if k8sProvider.IsAvailable() {
		aliases = []HostAliasProvider{k8sProvider}
	}

	return provider.Once(chain), aliases, nil
}
