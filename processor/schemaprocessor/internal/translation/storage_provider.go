// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"context"

	"go.opentelemetry.io/collector/extension/xextension/storage"
)

// StorageProvider wraps another Provider and adds persistent storage as a
// read-through cache. On Retrieve, it checks storage first. On a miss, it
// delegates to the wrapped provider and persists the result for future use.
type StorageProvider struct {
	wrapped Provider
	client  storage.Client
}

// NewStorageProvider creates a provider that checks persistent storage before
// delegating to the wrapped provider. Fetched schemas are persisted to storage
// for use across collector restarts.
func NewStorageProvider(wrapped Provider, client storage.Client) *StorageProvider {
	return &StorageProvider{
		wrapped: wrapped,
		client:  client,
	}
}

func (p *StorageProvider) Retrieve(ctx context.Context, schemaURL string) (string, error) {
	// Check persistent storage first.
	data, err := p.client.Get(ctx, schemaURL)
	if err == nil && len(data) > 0 {
		return string(data), nil
	}

	// Fall through to the wrapped provider (typically HTTP).
	content, err := p.wrapped.Retrieve(ctx, schemaURL)
	if err != nil {
		return "", err
	}

	// Persist to storage for next time. Best-effort — don't fail on storage errors.
	_ = p.client.Set(ctx, schemaURL, []byte(content))

	return content, nil
}
