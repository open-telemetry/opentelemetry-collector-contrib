// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package provider // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/provider"

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

var _ source.Provider = (*HostProvider)(nil)

type HostProvider string

func (p HostProvider) Source(context.Context) (source.Source, error) {
	return source.Source{Kind: source.HostnameKind, Identifier: string(p)}, nil
}

var _ source.Provider = (*ErrorSourceProvider)(nil)

type ErrorSourceProvider string

func (p ErrorSourceProvider) Source(context.Context) (source.Source, error) {
	return source.Source{}, errors.New(string(p))
}

var _ source.Provider = (*delayedProvider)(nil)

type delayedProvider struct {
	provider source.Provider
	delay    time.Duration
}

func (p *delayedProvider) Source(ctx context.Context) (source.Source, error) {
	time.Sleep(p.delay)
	return p.provider.Source(ctx)
}

func withDelay(provider source.Provider, delay time.Duration) source.Provider {
	return &delayedProvider{provider, delay}
}

func TestChain(t *testing.T) {
	tests := []struct {
		name         string
		providers    map[string]source.Provider
		priorityList []string

		buildErr string

		hostname string
		queryErr string
	}{
		{
			name: "missing provider in priority list",
			providers: map[string]source.Provider{
				"p1": HostProvider("p1SourceName"),
				"p2": ErrorSourceProvider("errP2"),
			},
			priorityList: []string{"p1", "p2", "p3"},

			buildErr: "\"p3\" source is not available in providers",
		},
		{
			name: "all providers fail",
			providers: map[string]source.Provider{
				"p1": ErrorSourceProvider("errP1"),
				"p2": ErrorSourceProvider("errP2"),
				"p3": HostProvider("p3SourceName"),
			},
			priorityList: []string{"p1", "p2"},

			queryErr: "no source provider was available",
		},
		{
			name: "no providers fail",
			providers: map[string]source.Provider{
				"p1": HostProvider("p1SourceName"),
				"p2": HostProvider("p2SourceName"),
				"p3": HostProvider("p3SourceName"),
			},
			priorityList: []string{"p1", "p2", "p3"},

			hostname: "p1SourceName",
		},
		{
			name: "some providers fail",
			providers: map[string]source.Provider{
				"p1": ErrorSourceProvider("p1Err"),
				"p2": HostProvider("p2SourceName"),
				"p3": ErrorSourceProvider("p3Err"),
			},
			priorityList: []string{"p1", "p2", "p3"},

			hostname: "p2SourceName",
		},
		{
			name: "p2 takes longer than p3",
			providers: map[string]source.Provider{
				"p1": ErrorSourceProvider("p1Err"),
				"p2": withDelay(HostProvider("p2SourceName"), 50*time.Millisecond),
				"p3": HostProvider("p3SourceName"),
			},
			priorityList: []string{"p1", "p2", "p3"},

			hostname: "p2SourceName",
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			provider, err := Chain(zaptest.NewLogger(t), testInstance.providers, testInstance.priorityList)
			if err != nil || testInstance.buildErr != "" {
				assert.EqualError(t, err, testInstance.buildErr)
				return
			}

			src, err := provider.Source(context.Background())
			if err != nil || testInstance.queryErr != "" {
				assert.EqualError(t, err, testInstance.queryErr)
			} else {
				assert.Equal(t, testInstance.hostname, src.Identifier)
			}
		})
	}
}
