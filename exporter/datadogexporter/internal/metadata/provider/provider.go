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

package provider // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/provider"

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// HostnameProvider of a hostname from a given place.
type HostnameProvider interface {
	// Metadata gets host metadata from provider.
	Hostname(ctx context.Context) (string, error)
}

var _ HostnameProvider = (*chainProvider)(nil)

type chainProvider struct {
	logger       *zap.Logger
	providers    map[string]HostnameProvider
	priorityList []string
}

func (p *chainProvider) Hostname(ctx context.Context) (string, error) {
	for _, source := range p.priorityList {
		zapSource := zap.String("source", source)
		provider := p.providers[source]
		hostname, err := provider.Hostname(ctx)
		if err == nil {
			p.logger.Info("Resolved hostname", zapSource, zap.String("hostname", hostname))
			return hostname, nil
		}
		p.logger.Debug("Unavailable hostname provider", zapSource, zap.Error(err))
	}

	return "", fmt.Errorf("no provider was available")
}

// Chain providers into a single provider that returns the first available hostname.
func Chain(logger *zap.Logger, providers map[string]HostnameProvider, priorityList []string) (HostnameProvider, error) {
	for _, source := range priorityList {
		if _, ok := providers[source]; !ok {
			return nil, fmt.Errorf("%q source is not available in providers", source)
		}
	}

	return &chainProvider{logger: logger, providers: providers, priorityList: priorityList}, nil
}

var _ HostnameProvider = (*configProvider)(nil)

type configProvider struct {
	hostname string
}

func (p *configProvider) Hostname(context.Context) (string, error) {
	if p.hostname == "" {
		return "", fmt.Errorf("invalid configuration hostname: %q", p.hostname)
	}
	return p.hostname, nil
}

// Config returns fixed hostname.
func Config(hostname string) HostnameProvider {
	return &configProvider{hostname}
}

var _ HostnameProvider = (*onceProvider)(nil)

type onceProvider struct {
	once     *sync.Once
	hostname string
	err      error
	provider HostnameProvider
}

func (c *onceProvider) Hostname(ctx context.Context) (string, error) {
	c.once.Do(func() {
		c.hostname, c.err = c.provider.Hostname(ctx)
	})

	return c.hostname, c.err
}

// Once wraps a provider to call it only once.
func Once(provider HostnameProvider) HostnameProvider {
	return &onceProvider{
		provider: provider,
	}
}
