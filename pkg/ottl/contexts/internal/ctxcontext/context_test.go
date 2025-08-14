// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxcontext

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func TestContextClientMetadata(t *testing.T) {
	clientMD := client.NewMetadata(map[string][]string{
		"auth":         {"Bearer token123"},
		"content-type": {"application/json"},
		"user-agent":   {"test-agent/1.0"},
		"empty-key":    {},
		"multi-values": {"value1", "value2"},
	})

	ctx := client.NewContext(context.Background(), client.Info{
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
		require.Equal(t, clientMD, val)

		result, ok := val.(client.Metadata)
		require.True(t, ok)

		auth := result.Get("auth")
		assert.Equal(t, []string{"Bearer token123"}, auth)

		contentType := result.Get("content-type")
		assert.Equal(t, []string{"application/json"}, contentType)
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
		assert.Equal(t, "Bearer token123", val)
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
		// Should return nil when there are multiple values
		assert.Nil(t, val)
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
		emptyCtx := context.Background()

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
					N: "context",
					NextPath: &pathtest.Path[testContext]{
						N: "client",
						NextPath: &pathtest.Path[testContext]{
							N: "metadata",
							KeySlice: []ottl.Key[testContext]{
								&pathtest.Key[testContext]{
									S: ottltest.Strp(tc.key),
								},
							},
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
	ctx := client.NewContext(context.Background(), client.Info{Addr: addr})

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
	ctx := client.NewContext(context.Background(), client.Info{Auth: auth})

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
		m, ok := val.(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "user-123", m["subject"])
		assert.Equal(t, "[\"admin\",\"user\"]", m["roles"]) // JSON stringified slice

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
		assert.Equal(t, "", val)
	})

	t.Run("attributes key without keys error", func(t *testing.T) {
		getter := accessAuthAttributesKey[testContext]([]ottl.Key[testContext]{})
		_, err := getter.Get(ctx, testContext{})
		require.Error(t, err)
		assert.Equal(t, "cannot get map value without keys", err.Error())
	})
}

func TestContextGrpcMetadata(t *testing.T) {
	parser := PathExpressionParser[testContext]()

	base := context.Background()
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
		mdVal, ok := val.(metadata.MD)
		require.True(t, ok)
		assert.Equal(t, []string{"v1", "v2"}, mdVal.Get("k1"))
		assert.Equal(t, []string{"only"}, mdVal.Get("single"))

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
		assert.Equal(t, []string{"v1", "v2"}, val)

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
		getter := accessGrpcMetadataContextKey[testContext]([]ottl.Key[testContext]{})
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
		val, err := getter.Get(context.Background(), testContext{})
		require.NoError(t, err)
		assert.Nil(t, val)
	})
}

type testContext struct{}

func (tCtx testContext) GetTestValue() string {
	return "test"
}

type testAddr struct{ s string }

func (a testAddr) Network() string { return "tcp" }
func (a testAddr) String() string  { return a.s }

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
