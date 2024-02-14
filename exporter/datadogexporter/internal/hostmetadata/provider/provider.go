// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package provider contains the cluster name provider
package provider // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/provider"

import (
	"context"
	"fmt"
	"sync"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"go.uber.org/zap"
)

var _ source.Provider = (*chainProvider)(nil)

type chainProvider struct {
	logger       *zap.Logger
	providers    map[string]source.Provider
	priorityList []string
}

func (p *chainProvider) Source(ctx context.Context) (source.Source, error) {
	// Auxiliary type for storing source provider replies
	type reply struct {
		src source.Source
		err error
	}

	// Cancel all providers when exiting
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Run all providers in parallel
	replies := make([]chan reply, len(p.priorityList))
	for i, source := range p.priorityList {
		provider := p.providers[source]
		replies[i] = make(chan reply)
		p.logger.Debug("Trying out source provider", zap.String("provider", source))
		go func(i int) {
			src, err := provider.Source(ctx)
			replies[i] <- reply{src: src, err: err}
		}(i)
	}

	// Check provider responses in order to ensure priority
	for i, ch := range replies {
		zapProvider := zap.String("provider", p.priorityList[i])
		select {
		case <-ctx.Done():
			return source.Source{}, fmt.Errorf("context was cancelled: %w", ctx.Err())
		case reply := <-ch:
			if reply.err != nil {
				p.logger.Debug("Unavailable source provider", zapProvider, zap.Error(reply.err))
				continue
			}

			p.logger.Info("Resolved source", zapProvider, zap.Any("source", reply.src))
			return reply.src, nil
		}
	}

	return source.Source{}, fmt.Errorf("no source provider was available")
}

// Chain providers into a single provider that returns the first available hostname.
func Chain(logger *zap.Logger, providers map[string]source.Provider, priorityList []string) (source.Provider, error) {
	for _, source := range priorityList {
		if _, ok := providers[source]; !ok {
			return nil, fmt.Errorf("%q source is not available in providers", source)
		}
	}

	return &chainProvider{logger: logger, providers: providers, priorityList: priorityList}, nil
}

var _ source.Provider = (*configProvider)(nil)

type configProvider struct {
	hostname string
}

func (p *configProvider) Source(context.Context) (source.Source, error) {
	if p.hostname == "" {
		return source.Source{}, fmt.Errorf("empty configuration hostname")
	}
	return source.Source{Kind: source.HostnameKind, Identifier: p.hostname}, nil
}

// Config returns fixed hostname.
func Config(hostname string) source.Provider {
	return &configProvider{hostname}
}

var _ source.Provider = (*onceProvider)(nil)

type onceProvider struct {
	once     sync.Once
	src      source.Source
	err      error
	provider source.Provider
}

func (c *onceProvider) Source(ctx context.Context) (source.Source, error) {
	c.once.Do(func() {
		c.src, c.err = c.provider.Source(ctx)
	})

	return c.src, c.err
}

// Once wraps a provider to call it only once.
func Once(provider source.Provider) source.Provider {
	return &onceProvider{
		provider: provider,
	}
}
