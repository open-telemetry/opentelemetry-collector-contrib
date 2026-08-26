// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextWithClient(t *testing.T) {
	client := &http.Client{}
	ctx := t.Context()

	ctxWithClient := ContextWithClient(ctx, client)

	// Verify context is not nil
	require.NotNil(t, ctxWithClient)

	// Verify we can retrieve the client
	retrievedClient, err := ClientFromContext(ctxWithClient)
	require.NoError(t, err)
	assert.Equal(t, client, retrievedClient)
}

func TestClientFromContext(t *testing.T) {
	t.Run("valid client in context", func(t *testing.T) {
		client := &http.Client{}
		ctx := ContextWithClient(t.Context(), client)

		retrievedClient, err := ClientFromContext(ctx)
		require.NoError(t, err)
		assert.Equal(t, client, retrievedClient)
	})

	t.Run("no client in context", func(t *testing.T) {
		ctx := t.Context()

		retrievedClient, err := ClientFromContext(ctx)
		assert.Error(t, err)
		assert.Nil(t, retrievedClient)
		assert.Contains(t, err.Error(), "no http.Client in context")
	})

	t.Run("invalid value in context", func(t *testing.T) {
		// Create a context with wrong type
		ctx := context.WithValue(t.Context(), clientContextKey, "not a client")

		retrievedClient, err := ClientFromContext(ctx)
		assert.Error(t, err)
		assert.Nil(t, retrievedClient)
		assert.Contains(t, err.Error(), "invalid value found in context")
	})

	t.Run("nil client", func(t *testing.T) {
		// This tests the edge case behavior
		ctx := t.Context()
		ctxWithNilClient := ContextWithClient(ctx, nil)

		retrievedClient, err := ClientFromContext(ctxWithNilClient)
		require.NoError(t, err)
		assert.Nil(t, retrievedClient)
	})
}

func TestContextKeyIsolation(t *testing.T) {
	// Verify that our context key doesn't interfere with other context values
	client := &http.Client{}

	type otherKey int
	const someOtherKey otherKey = 999

	ctx := t.Context()
	ctx = context.WithValue(ctx, someOtherKey, "some other value")
	ctx = ContextWithClient(ctx, client)

	// Both values should be retrievable
	assert.Equal(t, "some other value", ctx.Value(someOtherKey))
	retrievedClient, err := ClientFromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, client, retrievedClient)
}
