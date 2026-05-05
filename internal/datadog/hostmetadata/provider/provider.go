// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package provider contains the cluster name provider
package provider // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/provider"

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"go.uber.org/zap"
)

// SourceAliasesProvider is a source.Provider that also returns host aliases alongside the main source.
type SourceAliasesProvider interface {
	source.Provider
	SourceWithAliases(ctx context.Context) (source.Source, []string, error)
}

var _ SourceAliasesProvider = (*chainProvider)(nil)

type chainProvider struct {
	logger       *zap.Logger
	providers    map[string]source.Provider
	priorityList []string
	aliasedList  []string
	timeout      time.Duration
}

func (p *chainProvider) SourceWithAliases(ctx context.Context) (source.Source, []string, error) {
	// Auxiliary type for storing source provider replies
	type reply struct {
		src source.Source
		err error
	}

	// Cancel all providers when exiting
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Make a different context for our provider calls, to differentiate between a provider timing out and the entire
	// context being cancelled
	var childCtx context.Context
	if p.timeout != 0 {
		childCtx, cancel = context.WithTimeout(ctx, p.timeout)
	} else {
		childCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	var aliasesWg sync.WaitGroup
	var aliasesMu sync.Mutex
	var aliases []string

	// Run all providers in parallel
	replies := make([]chan reply, len(p.priorityList))
	for i, sourceName := range p.priorityList {
		provider := p.providers[sourceName]
		replies[i] = make(chan reply, 1) // Capacity required to avoid leaking goroutines / blocking aliasesWg
		p.logger.Debug("Trying out source provider", zap.String("provider", sourceName))
		isAliased := slices.Contains(p.aliasedList, sourceName)
		if isAliased {
			aliasesWg.Add(1)
		}
		go func(i int, isAliased bool) {
			if isAliased {
				defer aliasesWg.Done()
			}
			src, err := provider.Source(childCtx)
			if isAliased && err == nil && src.Kind == source.HostnameKind {
				aliasesMu.Lock()
				aliases = append(aliases, src.Identifier)
				aliasesMu.Unlock()
			}
			replies[i] <- reply{src: src, err: err}
		}(i, isAliased)
	}

	// Check provider responses in order to ensure priority
	for i, ch := range replies {
		zapProvider := zap.String("provider", p.priorityList[i])
		select {
		case <-ctx.Done():
			return source.Source{}, nil, fmt.Errorf("context was cancelled: %w", ctx.Err())
		case reply := <-ch:
			if reply.err != nil {
				p.logger.Debug("Unavailable source provider", zapProvider, zap.Error(reply.err))
				continue
			}

			aliasesWg.Wait()
			if reply.src.Kind == source.HostnameKind {
				aliases = slices.DeleteFunc(aliases, func(s string) bool {
					return s == reply.src.Identifier
				})
			}

			p.logger.Info("Resolved source", zapProvider, zap.Any("source", reply.src))
			return reply.src, aliases, nil
		}
	}

	return source.Source{}, nil, errors.New("no source provider was available")
}

func (p *chainProvider) Source(ctx context.Context) (source.Source, error) {
	src, _, err := p.SourceWithAliases(ctx)
	return src, err
}

// Chain providers into a single provider that returns the first available hostname.
// aliasedList contains providers whose hostname results are always awaited and added as aliases
// when not chosen as the main source.
func Chain(logger *zap.Logger, providers map[string]source.Provider, priorityList, aliasedList []string, timeout time.Duration) (SourceAliasesProvider, error) {
	for _, source := range priorityList {
		if _, ok := providers[source]; !ok {
			return nil, fmt.Errorf("%q source is not available in providers", source)
		}
	}
	for _, source := range aliasedList {
		if _, ok := providers[source]; !ok {
			return nil, fmt.Errorf("%q source is not available in providers", source)
		}
	}

	return &chainProvider{logger: logger, providers: providers, priorityList: priorityList, aliasedList: aliasedList, timeout: timeout}, nil
}

var _ source.Provider = (*configProvider)(nil)

type configProvider struct {
	hostname string
}

func (p *configProvider) Source(context.Context) (source.Source, error) {
	if p.hostname == "" {
		return source.Source{}, errors.New("empty configuration hostname")
	}
	return source.Source{Kind: source.HostnameKind, Identifier: p.hostname}, nil
}

// Config returns fixed hostname.
func Config(hostname string) source.Provider {
	return &configProvider{hostname}
}

var _ SourceAliasesProvider = (*onceProvider)(nil)

type onceProvider struct {
	once     sync.Once
	src      source.Source
	aliases  []string
	err      error
	provider SourceAliasesProvider
}

func (c *onceProvider) SourceWithAliases(ctx context.Context) (source.Source, []string, error) {
	c.once.Do(func() {
		c.src, c.aliases, c.err = c.provider.SourceWithAliases(ctx)
	})

	return c.src, c.aliases, c.err
}

func (c *onceProvider) Source(ctx context.Context) (source.Source, error) {
	src, _, err := c.SourceWithAliases(ctx)
	return src, err
}

// Once wraps a provider to call it only once.
func Once(provider SourceAliasesProvider) SourceAliasesProvider {
	return &onceProvider{
		provider: provider,
	}
}
