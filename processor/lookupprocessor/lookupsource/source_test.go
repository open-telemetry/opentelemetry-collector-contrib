// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestNewSource(t *testing.T) {
	lookupCalled := false
	source := NewSource(
		func(_ context.Context, key string) (any, bool, error) {
			lookupCalled = true
			return "value-for-" + key, true, nil
		},
		func() string { return "test" },
		nil,
		nil,
	)

	require.NotNil(t, source)
	assert.Equal(t, "test", source.Type())

	val, found, err := source.Lookup(t.Context(), "key1")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "value-for-key1", val)
	assert.True(t, lookupCalled)
}

func TestSourceStartShutdown(t *testing.T) {
	startCalled := false
	shutdownCalled := false

	source := NewSource(
		func(_ context.Context, _ string) (any, bool, error) {
			return nil, false, nil
		},
		func() string { return "test" },
		func(_ context.Context, _ component.Host) error {
			startCalled = true
			return nil
		},
		func(_ context.Context) error {
			shutdownCalled = true
			return nil
		},
	)

	host := componenttest.NewNopHost()
	require.NoError(t, source.Start(t.Context(), host))
	assert.True(t, startCalled)

	require.NoError(t, source.Shutdown(t.Context()))
	assert.True(t, shutdownCalled)
}

func TestSourceNilFunctions(t *testing.T) {
	// Source with nil functions should not panic
	source := NewSource(nil, nil, nil, nil)

	// Lookup returns (nil, false, nil) when function is nil
	val, found, err := source.Lookup(t.Context(), "key")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, val)

	// Type returns "unknown" when function is nil
	assert.Equal(t, "unknown", source.Type())

	// Start and Shutdown don't error when functions are nil
	host := componenttest.NewNopHost()
	require.NoError(t, source.Start(t.Context(), host))
	require.NoError(t, source.Shutdown(t.Context()))
}

func TestSourceLookupError(t *testing.T) {
	expectedErr := errors.New("lookup failed")
	source := NewSource(
		func(_ context.Context, _ string) (any, bool, error) {
			return nil, false, expectedErr
		},
		func() string { return "test" },
		nil,
		nil,
	)

	_, _, err := source.Lookup(t.Context(), "key")
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestSourceStartError(t *testing.T) {
	expectedErr := errors.New("start failed")
	source := NewSource(
		func(_ context.Context, _ string) (any, bool, error) {
			return nil, false, nil
		},
		func() string { return "test" },
		func(_ context.Context, _ component.Host) error {
			return expectedErr
		},
		nil,
	)

	host := componenttest.NewNopHost()
	err := source.Start(t.Context(), host)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}
