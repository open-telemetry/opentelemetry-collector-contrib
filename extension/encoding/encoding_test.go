// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecoderOptions(t *testing.T) {
	t.Run("Check Defaults", func(t *testing.T) {
		opts := NewDecoderOptions()

		assert.Equal(t, int64(defaultFlushBytes), opts.FlushBytes)
		assert.Equal(t, int64(defaultFlushItems), opts.FlushItems)
		assert.Equal(t, int64(0), opts.Offset)
		require.NotNil(t, opts.Context)
		assert.Nil(t, opts.Context.Done())
		require.NoError(t, opts.Context.Err())
	})

	t.Run("Check overrides", func(t *testing.T) {
		opts := NewDecoderOptions()
		WithFlushBytes(100)(&opts)
		WithFlushItems(50)(&opts)
		WithOffset(50)(&opts)

		assert.Equal(t, int64(100), opts.FlushBytes)
		assert.Equal(t, int64(50), opts.FlushItems)
		assert.Equal(t, int64(50), opts.Offset)
	})
}

func TestDecoderOptionsContext(t *testing.T) {
	t.Run("preserves the supplied context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		opts := NewDecoderOptions(WithContext(ctx))
		assert.Same(t, ctx, opts.Context)

		cancel()
		require.ErrorIs(t, opts.Context.Err(), context.Canceled)
	})

	t.Run("nil uses a background context", func(t *testing.T) {
		opts := NewDecoderOptions(WithContext(t.Context()), WithContext(nil)) //nolint:staticcheck // Verify the documented nil-context fallback.
		require.NotNil(t, opts.Context)
		assert.Nil(t, opts.Context.Done())
		require.NoError(t, opts.Context.Err())
	})
}
