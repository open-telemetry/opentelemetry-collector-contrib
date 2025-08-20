// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxcontext

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func TestContextClientMetadata(t *testing.T) {
	clientMDRaw := map[string][]string{
		"auth":         {"Bearer token123"},
		"content-type": {"application/json"},
		"user-agent":   {"test-agent/1.0"},
		"empty-key":    {},
		"multi-values": {"value1", "value2"},
	}
	clientMD := client.NewMetadata(clientMDRaw)

	ctx := client.NewContext(t.Context(), client.Info{
		Metadata: clientMD,
	})

	parser := PathExpressionParser[testContext]()

	t.Run("access entire metadata", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)

		result, ok := val.(pcommon.Map)
		require.True(t, ok)

		auth, ok := result.Get("auth")
		require.True(t, ok)
		require.Equal(t, []any{"Bearer token123"}, auth.Slice().AsRaw())

		contentType, ok := result.Get("content-type")
		require.True(t, ok)
		require.Equal(t, []any{"application/json"}, contentType.Slice().AsRaw())
	})

	t.Run("access specific metadata key", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
					KeySlice: []ottl.Key[testContext]{
						&pathtest.Key[testContext]{
							S: ottltest.Strp("auth"),
						},
					},
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)
		result, ok := val.(pcommon.Slice)
		require.True(t, ok)
		assert.Equal(t, []any{"Bearer token123"}, result.AsRaw())
	})

	t.Run("access non-existent metadata key", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
					KeySlice: []ottl.Key[testContext]{
						&pathtest.Key[testContext]{
							S: ottltest.Strp("non-existent"),
						},
					},
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
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
					KeySlice: []ottl.Key[testContext]{
						&pathtest.Key[testContext]{
							S: ottltest.Strp("empty-key"),
						},
					},
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
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
					KeySlice: []ottl.Key[testContext]{
						&pathtest.Key[testContext]{
							S: ottltest.Strp("multi-values"),
						},
					},
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)
		result, ok := val.(pcommon.Slice)
		require.True(t, ok)
		assert.Equal(t, []any{"value1", "value2"}, result.AsRaw())
	})

	t.Run("access metadata key with multiple values by index", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
					KeySlice: []ottl.Key[testContext]{
						&pathtest.Key[testContext]{
							S: ottltest.Strp("multi-values"),
						},
						&pathtest.Key[testContext]{
							I: ottltest.Intp(0),
						},
					},
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)
		result, ok := val.(string)
		require.True(t, ok)
		assert.Equal(t, "value1", result)
	})

	t.Run("cannot set entire metadata", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		newMetadata := client.NewMetadata(map[string][]string{
			"new-key": {"new-value"},
		})

		err = getter.Set(ctx, testContext{}, newMetadata)
		require.Error(t, err)
		assert.Equal(t, "cannot set value in context.client.metadata", err.Error())
	})

	t.Run("cannot set specific metadata key", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
					KeySlice: []ottl.Key[testContext]{
						&pathtest.Key[testContext]{
							S: ottltest.Strp("auth"),
						},
					},
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		err = getter.Set(ctx, testContext{}, "new-value")
		require.Error(t, err)
		assert.Equal(t, "cannot set value in context.client.metadata", err.Error())
	})

	t.Run("error when accessing metadata key without keys", func(t *testing.T) {
		// This should not happen through normal parser, but testing the underlying function
		getter := accessClientMetadataKey[testContext]([]ottl.Key[testContext]{})

		_, err := getter.Get(ctx, testContext{})
		require.Error(t, err)
		assert.Equal(t, "cannot get map value without keys", err.Error())
	})

	t.Run("no client in context", func(t *testing.T) {
		// Test with context that has no client info
		emptyCtx := t.Context()

		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(emptyCtx, testContext{})
		require.NoError(t, err)
		// Should return empty pcommon.Map when no client is in context
		emptyMap := pcommon.NewMap()
		assert.Equal(t, emptyMap, val)
	})
}

func TestContextClientAddr(t *testing.T) {
	parser := PathExpressionParser[testContext]()

	path := &pathtest.Path[testContext]{
		N: "context",
		NextPath: &pathtest.Path[testContext]{
			N: "client",
			NextPath: &pathtest.Path[testContext]{
				N: "addr",
			},
		},
	}

	getter, err := parser(path)
	require.NoError(t, err)

	addr := testAddr{"127.0.0.1:4317"}
	ctx := client.NewContext(t.Context(), client.Info{Addr: addr})

	val, err := getter.Get(ctx, testContext{})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:4317", val)

	err = getter.Set(ctx, testContext{}, "ignored")
	require.Error(t, err)
	assert.Equal(t, "cannot set value in context.client.addr", err.Error())
}

func TestContextClientAuthAttributes_AllAndKey(t *testing.T) {
	parser := PathExpressionParser[testContext]()

	auth := testAuth{
		attrs: map[string]any{
			"subject": "user-123",
			"roles":   []string{"admin", "user"},
		},
	}
	ctx := client.NewContext(t.Context(), client.Info{Auth: auth})

	t.Run("all attributes map", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "auth",
					NextPath: &pathtest.Path[testContext]{
						N: "attributes",
					},
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)
		m, ok := val.(pcommon.Map)
		require.True(t, ok)
		user, ok := m.Get("subject")
		require.True(t, ok)
		assert.Equal(t, "user-123", user.AsString())
		roles, ok := m.Get("roles")
		require.True(t, ok)
		assert.Equal(t, "[\"admin\",\"user\"]", roles.AsString())

		err = getter.Set(ctx, testContext{}, map[string]string{"k": "v"})
		require.Error(t, err)
		assert.Equal(t, "cannot set value in context.client.auth.attributes", err.Error())
	})

	t.Run("specific attribute key present", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "auth",
					NextPath: &pathtest.Path[testContext]{
						N: "attributes",
						KeySlice: []ottl.Key[testContext]{
							&pathtest.Key[testContext]{S: ottltest.Strp("subject")},
						},
					},
				},
			},
		}
		getter, err := parser(path)
		require.NoError(t, err)
		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)
		assert.Equal(t, "user-123", val)
	})

	t.Run("specific attribute key missing returns empty string", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "client",
				NextPath: &pathtest.Path[testContext]{
					N: "auth",
					NextPath: &pathtest.Path[testContext]{
						N: "attributes",
						KeySlice: []ottl.Key[testContext]{
							&pathtest.Key[testContext]{S: ottltest.Strp("missing")},
						},
					},
				},
			},
		}
		getter, err := parser(path)
		require.NoError(t, err)
		val, err := getter.Get(ctx, testContext{})
		require.NoError(t, err)
		assert.Empty(t, val)
	})

	t.Run("attributes key without keys error", func(t *testing.T) {
		getter := accessClientAuthAttributesKey[testContext]([]ottl.Key[testContext]{})
		_, err := getter.Get(ctx, testContext{})
		require.Error(t, err)
		assert.Equal(t, "cannot get map value without keys", err.Error())
	})
}

func TestContextGrpcMetadata(t *testing.T) {
	parser := PathExpressionParser[testContext]()

	base := t.Context()
	// include client context too, to ensure coexistence
	base = client.NewContext(base, client.Info{})

	md := metadata.Pairs(
		"k1", "v1",
		"k1", "v2",
		"single", "only",
	)
	ctxWithMD := metadata.NewIncomingContext(base, md)

	t.Run("get entire grpc metadata", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "grpc",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
				},
			},
		}
		getter, err := parser(path)
		require.NoError(t, err)
		val, err := getter.Get(ctxWithMD, testContext{})
		require.NoError(t, err)
		mdVal, ok := val.(pcommon.Map)
		require.True(t, ok)
		k, ok := mdVal.Get("k1")
		require.True(t, ok)
		assert.Equal(t, []any{"v1", "v2"}, k.Slice().AsRaw())

		s, ok := mdVal.Get("single")
		require.True(t, ok)
		assert.Equal(t, []any{"only"}, s.Slice().AsRaw())

		err = getter.Set(ctxWithMD, testContext{}, metadata.MD{})
		require.Error(t, err)
		assert.Equal(t, "cannot set value in context.grpc.metadata", err.Error())
	})

	t.Run("get specific grpc metadata key values", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "grpc",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
					KeySlice: []ottl.Key[testContext]{
						&pathtest.Key[testContext]{S: ottltest.Strp("k1")},
					},
				},
			},
		}
		getter, err := parser(path)
		require.NoError(t, err)
		val, err := getter.Get(ctxWithMD, testContext{})
		require.NoError(t, err)
		sl, ok := val.(pcommon.Slice)
		require.True(t, ok)
		assert.Equal(t, []any{"v1", "v2"}, sl.AsRaw())

		err = getter.Set(ctxWithMD, testContext{}, []string{"x"})
		require.Error(t, err)
		assert.Equal(t, "cannot set value in context.grpc.metadata", err.Error())
	})

	t.Run("grpc metadata key missing returns nil", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "grpc",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
					KeySlice: []ottl.Key[testContext]{
						&pathtest.Key[testContext]{S: ottltest.Strp("missing")},
					},
				},
			},
		}
		getter, err := parser(path)
		require.NoError(t, err)
		val, err := getter.Get(ctxWithMD, testContext{})
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("grpc metadata without keys error", func(t *testing.T) {
		getter := accessGRPCMetadataKey[testContext]([]ottl.Key[testContext]{})
		_, err := getter.Get(ctxWithMD, testContext{})
		require.Error(t, err)
		assert.Equal(t, "cannot get map value without keys", err.Error())
	})

	t.Run("no grpc metadata in context returns nil", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "context",
			NextPath: &pathtest.Path[testContext]{
				N: "grpc",
				NextPath: &pathtest.Path[testContext]{
					N: "metadata",
				},
			},
		}
		getter, err := parser(path)
		require.NoError(t, err)
		val, err := getter.Get(t.Context(), testContext{})
		require.NoError(t, err)
		assert.Nil(t, val)
	})
}

type testContext struct{}

func (testContext) GetTestValue() string {
	return "test"
}

type testAddr struct{ s string }

func (testAddr) Network() string  { return "tcp" }
func (a testAddr) String() string { return a.s }

var _ net.Addr = (*testAddr)(nil)

type testAuth struct{ attrs map[string]any }

func (a testAuth) GetAttribute(name string) any {
	return a.attrs[name]
}

func (a testAuth) GetAttributeNames() []string {
	names := make([]string, 0, len(a.attrs))
	for k := range a.attrs {
		names = append(names, k)
	}
	return names
}
