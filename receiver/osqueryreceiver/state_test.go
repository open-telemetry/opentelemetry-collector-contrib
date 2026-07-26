// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

var testComponentID = component.MustNewID("osquery")

func TestGetStorageClient_NilID(t *testing.T) {
	host := storagetest.NewStorageHost()
	client, err := getStorageClient(t.Context(), host, nil, testComponentID)
	require.NoError(t, err)
	assert.Equal(t, storage.NewNopClient(), client)
}

func TestGetStorageClient_NotFound(t *testing.T) {
	host := storagetest.NewStorageHost()
	id := storagetest.NewStorageID("missing")
	_, err := getStorageClient(t.Context(), host, &id, testComponentID)
	require.ErrorContains(t, err, "not found")
}

func TestGetStorageClient_NonStorageExtension(t *testing.T) {
	host := storagetest.NewStorageHost().WithNonStorageExtension("not_storage")
	id := storagetest.NewNonStorageID("not_storage")
	_, err := getStorageClient(t.Context(), host, &id, testComponentID)
	require.ErrorContains(t, err, "non-storage extension")
}

func TestGetStorageClient_InMemoryExtension(t *testing.T) {
	host := storagetest.NewStorageHost().WithInMemoryStorageExtension("test")
	id := storagetest.NewStorageID("test")
	client, err := getStorageClient(t.Context(), host, &id, testComponentID)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NoError(t, client.Close(t.Context()))
}

func TestCollectionState_NilSafe(t *testing.T) {
	var s *collectionState

	rows, err := s.load(t.Context(), "system_info")
	require.NoError(t, err)
	assert.Nil(t, rows)

	s.save(t.Context(), "system_info", []map[string]string{{"hostname": "test"}})
	require.NoError(t, s.close(t.Context()))
}

func TestCollectionState_InMemoryRoundTrip(t *testing.T) {
	s := newCollectionState(storage.NewNopClient(), zap.NewNop())

	rows, err := s.load(t.Context(), "package_info")
	require.NoError(t, err)
	assert.Nil(t, rows)

	saved := []map[string]string{{"name": "curl", "version": "8.0"}}
	s.save(t.Context(), "package_info", saved)

	rows, err = s.load(t.Context(), "package_info")
	require.NoError(t, err)
	assert.Equal(t, saved, rows)
}

func TestCollectionState_PersistsAcrossInstances(t *testing.T) {
	host := storagetest.NewStorageHost().WithInMemoryStorageExtension("test")
	id := storagetest.NewStorageID("test")
	client, err := getStorageClient(t.Context(), host, &id, testComponentID)
	require.NoError(t, err)

	first := newCollectionState(client, zap.NewNop())
	saved := []map[string]string{{"username": "alice"}}
	first.save(t.Context(), "users_info", saved)

	// A second collectionState backed by the same underlying storage client
	// (as would happen across a process restart) should load what the first
	// one saved, without relying on the first instance's in-memory cache.
	second := newCollectionState(client, zap.NewNop())
	rows, err := second.load(t.Context(), "users_info")
	require.NoError(t, err)
	assert.Equal(t, saved, rows)
}
