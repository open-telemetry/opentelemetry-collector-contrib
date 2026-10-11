// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package storagetest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
)

func TestInMemoryWalk(t *testing.T) {
	id := component.NewID(testStorageType)
	createClient := func(_, name string) *TestClient {
		return NewInMemoryClient(component.KindReceiver, id, sanitizeName(name))
	}
	runWalkTest(t, createClient)
}

func TestFileBackedWalk(t *testing.T) {
	id := component.NewID(testStorageType)
	createClient := func(storageDir, name string) *TestClient {
		return NewFileBackedClient(component.KindReceiver, id, sanitizeName(name), storageDir)
	}
	runWalkTest(t, createClient)
}

// Subtest names contain '/', which is not valid in the storage file path.
func sanitizeName(name string) string {
	return strings.ReplaceAll(name, "/", "_")
}

func runWalkTest(t *testing.T, createClient func(storageDir, name string) *TestClient) {
	seed := func(ctx context.Context, client *TestClient) error {
		return errors.Join(
			client.Set(ctx, "a", []byte("foo")),
			client.Set(ctx, "b", []byte("bar")),
			client.Set(ctx, "c", []byte("beep")),
			client.Set(ctx, "d", []byte("boop")),
		)
	}

	requireVal := func(t *testing.T, client *TestClient, key string, val []byte) {
		t.Helper()
		got, err := client.Get(t.Context(), key)
		require.NoError(t, err)
		require.Equal(t, val, got)
	}

	t.Run("empty", func(t *testing.T) {
		ctx := t.Context()
		client := createClient(t.TempDir(), t.Name())

		count := 0
		err := client.Walk(ctx, func(_ string, _ []byte) ([]*storage.Operation, error) {
			count++
			return nil, nil
		})
		require.Zero(t, count)
		require.NoError(t, err)
	})

	t.Run("ops_applied", func(t *testing.T) {
		ctx := t.Context()
		client := createClient(t.TempDir(), t.Name())

		require.NoError(t, seed(ctx, client))
		err := client.Walk(ctx, func(k string, _ []byte) ([]*storage.Operation, error) {
			return []*storage.Operation{storage.SetOperation(k, []byte("updated"))}, nil
		})
		require.NoError(t, err)
		requireVal(t, client, "a", []byte("updated"))
		requireVal(t, client, "b", []byte("updated"))
		requireVal(t, client, "c", []byte("updated"))
		requireVal(t, client, "d", []byte("updated"))
	})

	t.Run("stop_on_skip_all", func(t *testing.T) {
		ctx := t.Context()
		client := createClient(t.TempDir(), t.Name())

		require.NoError(t, seed(ctx, client))

		var keys []string
		// walk until we hit c
		err := client.Walk(ctx, func(k string, _ []byte) ([]*storage.Operation, error) {
			if k == "c" {
				return nil, storage.SkipAll
			}
			keys = append(keys, k)
			return nil, nil
		})
		require.NoError(t, err)
		require.Equal(t, []string{"a", "b"}, keys)
	})

	t.Run("error_from_callback", func(t *testing.T) {
		ctx := t.Context()
		client := createClient(t.TempDir(), t.Name())

		require.NoError(t, seed(ctx, client))

		testErr := errors.New("boom")
		err := client.Walk(ctx, func(_ string, _ []byte) ([]*storage.Operation, error) {
			return nil, testErr
		})
		require.ErrorIs(t, err, testErr)
	})

	t.Run("ops_not_applied_on_callback_error", func(t *testing.T) {
		ctx := t.Context()
		client := createClient(t.TempDir(), t.Name())

		require.NoError(t, seed(ctx, client))

		testErr := errors.New("boom")
		err := client.Walk(ctx, func(k string, _ []byte) ([]*storage.Operation, error) {
			if k == "d" {
				return []*storage.Operation{storage.DeleteOperation(k)}, testErr
			}
			return []*storage.Operation{storage.DeleteOperation(k)}, nil
		})
		require.ErrorIs(t, err, testErr)
		requireVal(t, client, "a", []byte("foo"))
		requireVal(t, client, "b", []byte("bar"))
		requireVal(t, client, "c", []byte("beep"))
		requireVal(t, client, "d", []byte("boop"))
	})

	t.Run("ops_not_applied_after_skip_all", func(t *testing.T) {
		ctx := t.Context()
		client := createClient(t.TempDir(), t.Name())

		require.NoError(t, seed(ctx, client))

		err := client.Walk(ctx, func(k string, _ []byte) ([]*storage.Operation, error) {
			if k == "c" {
				return []*storage.Operation{storage.DeleteOperation(k)}, storage.SkipAll
			}
			return []*storage.Operation{storage.DeleteOperation(k)}, nil
		})
		require.NoError(t, err)
		requireVal(t, client, "a", nil)
		requireVal(t, client, "b", nil)
		requireVal(t, client, "c", nil)
		requireVal(t, client, "d", []byte("boop"))
	})
}
