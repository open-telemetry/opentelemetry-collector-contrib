// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"context"

	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

// StorageProvider wraps another Provider and adds persistent storage as a
// read-through cache. On Retrieve, it checks storage first. On a miss, it
// delegates to the wrapped provider and persists the result for future use.
type StorageProvider struct {
	wrapped Provider
	client  storage.Client
	log     *zap.Logger
}

// NewStorageProvider creates a provider that checks persistent storage before
// delegating to the wrapped provider. Fetched schemas are persisted to storage
// for use across collector restarts.
func NewStorageProvider(wrapped Provider, client storage.Client, log *zap.Logger) *StorageProvider {
	return &StorageProvider{
		wrapped: wrapped,
		client:  client,
		log:     log,
	}
}

func (p *StorageProvider) Retrieve(ctx context.Context, schemaURL string) (string, error) {
	// Check persistent storage first.
	data, err := p.client.Get(ctx, schemaURL)
	if err == nil && len(data) > 0 {
		p.log.Debug("schema loaded from storage", zap.String("url", schemaURL))
		return string(data), nil
	}
	if err != nil {
		p.log.Debug("storage read failed, falling back to HTTP", zap.String("url", schemaURL), zap.Error(err))
	}

	// Fall through to the wrapped provider (typically HTTP).
	content, err := p.wrapped.Retrieve(ctx, schemaURL)
	if err != nil {
		return "", err
	}

	if content == "" {
		p.log.Warn("schema URL returned empty content, skipping storage cache", zap.String("url", schemaURL))
		return "", nil
	}

	// Persist to storage for next time. Best-effort — don't fail on storage errors.
	if err := p.client.Set(ctx, schemaURL, []byte(content)); err != nil {
		p.log.Warn("failed to persist schema to storage", zap.String("url", schemaURL), zap.Error(err))
	}

	return content, nil
}
