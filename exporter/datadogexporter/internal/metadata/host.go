// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/internal/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/internal/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/internal/ecs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/internal/gcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/internal/system"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/provider"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/valid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/source"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/utils/cache"
)

// UsePreviewHostnameLogic decides whether to use the preview hostname logic or not.
const UsePreviewHostnameLogic = false

func buildPreviewProvider(set component.TelemetrySettings, configHostname string) (source.Provider, error) {
	dockerProvider, err := docker.NewProvider()
	if err != nil {
		return nil, err
	}

	ecs, err := ecs.NewProvider(set)
	if err != nil {
		return nil, fmt.Errorf("failed to build ECS Fargate provider: %w", err)
	}

	chain, err := provider.Chain(
		set.Logger,
		map[string]source.Provider{
			"config": provider.Config(configHostname),
			"docker": dockerProvider,
			"azure":  azure.NewProvider(),
			"ecs":    ecs,
			"ec2":    ec2.NewProvider(set.Logger),
			"gcp":    gcp.NewProvider(),
			"system": system.NewProvider(set.Logger),
		},
		[]string{"config", "docker", "azure", "ecs", "ec2", "gcp", "system"},
	)

	if err != nil {
		return nil, err
	}

	return provider.Once(chain), nil
}

func buildCurrentProvider(set component.TelemetrySettings, configHostname string) (source.Provider, error) {
	return &currentProvider{
		logger:         set.Logger,
		configHostname: configHostname,
		systemProvider: system.NewProvider(set.Logger),
		ec2Provider:    ec2.NewProvider(set.Logger),
	}, nil
}

func GetHostnameProvider(set component.TelemetrySettings, configHostname string) (source.Provider, error) {
	if UsePreviewHostnameLogic {
		return buildPreviewProvider(set, configHostname)
	}

	return buildCurrentProvider(set, configHostname)
}

var _ source.Provider = (*currentProvider)(nil)

type currentProvider struct {
	logger         *zap.Logger
	configHostname string
	systemProvider *system.Provider
	ec2Provider    *ec2.Provider
}

// Hostname gets the hostname according to configuration.
// It checks in the following order
// 1. Configuration
// 2. Cache
// 3. EC2 instance metadata
// 4. System
func (c *currentProvider) hostname(ctx context.Context) string {
	if c.configHostname != "" {
		return c.configHostname
	}

	if cacheVal, ok := cache.Cache.Get(cache.CanonicalHostnameKey); ok {
		return cacheVal.(string)
	}

	ec2Info := c.ec2Provider.HostInfo()
	hostname := ec2Info.GetHostname(c.logger)

	if hostname == "" {
		// Get system hostname
		var err error
		src, err := c.systemProvider.Source(ctx)
		if err != nil {
			c.logger.Debug("system provider is unavailable", zap.Error(err))
		} else {
			hostname = src.Identifier
		}
	}

	if err := valid.Hostname(hostname); err != nil {
		// If invalid log but continue
		c.logger.Error("Detected hostname is not valid", zap.Error(err))
	}

	c.logger.Debug("Canonical hostname automatically set", zap.String("hostname", hostname))
	cache.Cache.Set(cache.CanonicalHostnameKey, hostname, cache.NoExpiration)
	return hostname
}

func (c *currentProvider) Source(ctx context.Context) (source.Source, error) {
	return source.Source{Kind: source.HostnameKind, Identifier: c.hostname(ctx)}, nil
}
