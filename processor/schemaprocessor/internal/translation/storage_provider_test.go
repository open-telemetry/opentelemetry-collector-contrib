// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

type recordingProvider struct {
	content string
	err     error
	called  bool
}

func (p *recordingProvider) Retrieve(_ context.Context, _ string) (string, error) {
	p.called = true
	return p.content, p.err
}

func TestStorageProviderHit(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindProcessor, component.ID{}, "test")
	require.NoError(t, client.Set(t.Context(), "http://example.com/schemas/1.0.0", []byte("cached content")))

	wrapped := &recordingProvider{content: "http content"}
	sp := NewStorageProvider(wrapped, client, zaptest.NewLogger(t))

	result, err := sp.Retrieve(t.Context(), "http://example.com/schemas/1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "cached content", result)
	assert.False(t, wrapped.called, "wrapped provider should not be called on storage hit")
}

func TestStorageProviderMiss(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindProcessor, component.ID{}, "test")

	wrapped := &recordingProvider{content: "http content"}
	sp := NewStorageProvider(wrapped, client, zaptest.NewLogger(t))

	result, err := sp.Retrieve(t.Context(), "http://example.com/schemas/1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "http content", result)
	assert.True(t, wrapped.called, "wrapped provider should be called on storage miss")

	// Verify it was persisted to storage.
	data, err := client.Get(t.Context(), "http://example.com/schemas/1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "http content", string(data))
}

func TestStorageProviderEmptyContent(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindProcessor, component.ID{}, "test")

	wrapped := &recordingProvider{content: ""}
	sp := NewStorageProvider(wrapped, client, zaptest.NewLogger(t))

	result, err := sp.Retrieve(t.Context(), "http://example.com/schemas/1.0.0")
	require.NoError(t, err)
	assert.Empty(t, result)
	assert.True(t, wrapped.called, "wrapped provider should be called on storage miss")

	// Verify nothing was persisted to storage.
	data, err := client.Get(t.Context(), "http://example.com/schemas/1.0.0")
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestStorageProviderWrappedError(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindProcessor, component.ID{}, "test")

	wrapped := &recordingProvider{err: errors.New("fetch failed")}
	sp := NewStorageProvider(wrapped, client, zaptest.NewLogger(t))

	_, err := sp.Retrieve(t.Context(), "http://example.com/schemas/1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch failed")

	// Verify nothing was persisted to storage.
	data, err := client.Get(t.Context(), "http://example.com/schemas/1.0.0")
	require.NoError(t, err)
	assert.Nil(t, data)
}
