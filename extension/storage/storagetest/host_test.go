// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package storagetest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func TestStorageHostWithNone(t *testing.T) {
	require.Equal(t, 0, len(NewStorageHost().GetExtensions()))
}

func TestStorageHostWithOne(t *testing.T) {
	storageID := component.NewIDWithName(testStorageType, "one")

	host := NewStorageHost().WithInMemoryStorageExtension("one")

	exts := host.GetExtensions()
	require.Equal(t, 1, len(exts))

	extOne, exists := exts[storageID]
	require.True(t, exists)

	storageOne, ok := extOne.(*TestStorage)
	require.True(t, ok)
	require.Equal(t, storageID, storageOne.ID)
}

func TestStorageHostWithTwo(t *testing.T) {
	storageOneID := component.NewIDWithName(testStorageType, "one")
	storageTwoID := component.NewIDWithName(testStorageType, "two")

	host := NewStorageHost().
		WithInMemoryStorageExtension("one").
		WithFileBackedStorageExtension("two", t.TempDir())

	exts := host.GetExtensions()
	require.Equal(t, 2, len(exts))

	extOne, exists := exts[storageOneID]
	require.True(t, exists)

	storageOne, ok := extOne.(*TestStorage)
	require.True(t, ok)
	require.Equal(t, storageOneID, storageOne.ID)

	extTwo, exists := exts[storageTwoID]
	require.True(t, exists)

	storageTwo, ok := extTwo.(*TestStorage)
	require.True(t, ok)
	require.Equal(t, storageTwoID, storageTwo.ID)
}

func TestStorageHostWithMixed(t *testing.T) {
	storageOneID := component.NewIDWithName(testStorageType, "one")
	storageTwoID := component.NewIDWithName(testStorageType, "two")
	nonStorageID := component.NewIDWithName(nonStorageType, "non-storage")

	host := NewStorageHost().
		WithInMemoryStorageExtension("one").
		WithFileBackedStorageExtension("two", t.TempDir()).
		WithNonStorageExtension("non-storage")

	exts := host.GetExtensions()
	require.Equal(t, 3, len(exts))

	extOne, exists := exts[storageOneID]
	require.True(t, exists)

	storageOne, ok := extOne.(*TestStorage)
	require.True(t, ok)
	require.Equal(t, storageOneID, storageOne.ID)

	extTwo, exists := exts[storageTwoID]
	require.True(t, exists)

	storageTwo, ok := extTwo.(*TestStorage)
	require.True(t, ok)
	require.Equal(t, storageTwoID, storageTwo.ID)

	extNon, exists := exts[nonStorageID]
	require.True(t, exists)

	nonStorage, ok := extNon.(*NonStorage)
	require.True(t, ok)
	require.Equal(t, nonStorageID, nonStorage.ID)
}
