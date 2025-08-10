// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxmetadata

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_PathExpressionParser(t *testing.T) {
	metadata := client.NewMetadata(map[string][]string{
		"auth":         {"Bearer token123"},
		"content-type": {"application/json"},
		"user-agent":   {"test-agent/1.0"},
		"empty-key":    {},
		"multi-values": {"value1", "value2"},
	})

	ctx := client.NewContext(context.Background(), client.Info{
		Metadata: metadata,
	})

	parser := PathExpressionParser[testContext]()

	t.Run("access entire metadata", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "metadata",
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)
		require.Equal(t, metadata, val)

		result, ok := val.(client.Metadata)
		require.True(t, ok)

		auth := result.Get("auth")
		assert.Equal(t, []string{"Bearer token123"}, auth)

		contentType := result.Get("content-type")
		assert.Equal(t, []string{"application/json"}, contentType)
	})

	t.Run("access specific metadata key", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "metadata",
			KeySlice: []ottl.Key[testContext]{
				&pathtest.Key[testContext]{
					S: ottltest.Strp("auth"),
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)
		assert.Equal(t, "Bearer token123", val)
	})

	t.Run("access non-existent metadata key", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "metadata",
			KeySlice: []ottl.Key[testContext]{
				&pathtest.Key[testContext]{
					S: ottltest.Strp("non-existent"),
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("access empty metadata key", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "metadata",
			KeySlice: []ottl.Key[testContext]{
				&pathtest.Key[testContext]{
					S: ottltest.Strp("empty-key"),
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("access metadata key with multiple values", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "metadata",
			KeySlice: []ottl.Key[testContext]{
				&pathtest.Key[testContext]{
					S: ottltest.Strp("multi-values"),
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)
		// Should return nil when there are multiple values
		assert.Nil(t, val)
	})

	t.Run("cannot set entire metadata", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "metadata",
		}

		getter, err := parser(path)
		require.NoError(t, err)

		newMetadata := client.NewMetadata(map[string][]string{
			"new-key": {"new-value"},
		})

		err = getter.Set(ctx, testContext{}, newMetadata)
		require.Error(t, err)
		assert.Equal(t, "cannot set value in metadata", err.Error())
	})

	t.Run("cannot set specific metadata key", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "metadata",
			KeySlice: []ottl.Key[testContext]{
				&pathtest.Key[testContext]{
					S: ottltest.Strp("auth"),
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		err = getter.Set(ctx, testContext{}, "new-value")
		require.Error(t, err)
		assert.Equal(t, "cannot set value in metadata", err.Error())
	})

	t.Run("error when accessing metadata key without keys", func(t *testing.T) {
		// This should not happen through normal parser, but testing the underlying function
		getter := accessMetadataKey[testContext]([]ottl.Key[testContext]{})

		_, err := getter.Get(ctx, testContext{})
		require.Error(t, err)
		assert.Equal(t, "cannot get map value without keys", err.Error())
	})

	t.Run("no client in context", func(t *testing.T) {
		// Test with context that has no client info
		emptyCtx := context.Background()

		path := &pathtest.Path[testContext]{
			N: "metadata",
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(emptyCtx, testContext{})
		require.NoError(t, err)
		// Should return empty metadata when no client is in context
		emptyMetadata := client.Metadata{}
		assert.Equal(t, emptyMetadata, val)
	})

	t.Run("access different metadata keys", func(t *testing.T) {
		testCases := []struct {
			name     string
			key      string
			expected string
		}{
			{
				name:     "content-type header",
				key:      "content-type",
				expected: "application/json",
			},
			{
				name:     "user-agent header",
				key:      "user-agent",
				expected: "test-agent/1.0",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				path := &pathtest.Path[testContext]{
					N: "metadata",
					KeySlice: []ottl.Key[testContext]{
						&pathtest.Key[testContext]{
							S: ottltest.Strp(tc.key),
						},
					},
				}

				getter, err := parser(path)
				require.NoError(t, err)

				val, err := getter.Get(ctx, testContext{})
				require.NoError(t, err)
				assert.Equal(t, tc.expected, val)
			})
		}
	})
}

type testContext struct{}

func (tCtx testContext) GetTestValue() string {
	return "test"
}
