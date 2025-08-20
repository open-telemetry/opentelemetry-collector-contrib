package ottlcontext

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"slices"
	"testing"
)

func Test_newPathGetSetter(t *testing.T) {
	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	tests := []struct {
		name         string
		path         ottl.Path[TransformContext]
		orig         any
		newVal       any
		wantErrOnSet bool
	}{
		{
			name: "cache",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
			},
			orig:         pcommon.NewMap(),
			newVal:       newCache,
			wantErrOnSet: false,
		},
		{
			name: "cache access",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("temp"),
					},
				},
			},
			orig:         nil,
			newVal:       "new value",
			wantErrOnSet: false,
		},
		{
			name: "client.addr",
			path: &pathtest.Path[TransformContext]{
				N: "client",
				NextPath: &pathtest.Path[TransformContext]{
					N: "addr",
				},
			},
			orig:         nil,
			newVal:       nil,
			wantErrOnSet: true,
		},
		{
			name: "client.metadata",
			path: &pathtest.Path[TransformContext]{
				N: "client",
				NextPath: &pathtest.Path[TransformContext]{
					N: "metadata",
				},
			},
			orig:         pcommon.NewMap(),
			newVal:       nil,
			wantErrOnSet: true,
		},
		{
			name: `client.metadata["key"]`,
			path: &pathtest.Path[TransformContext]{
				N: "client",
				NextPath: &pathtest.Path[TransformContext]{
					N: "metadata",
					KeySlice: []ottl.Key[TransformContext]{
						&pathtest.Key[TransformContext]{
							S: ottltest.Strp("key"),
						},
					},
				},
			},
			orig:         nil,
			newVal:       nil,
			wantErrOnSet: true,
		},
		{
			name: `client.auth.attributes`,
			path: &pathtest.Path[TransformContext]{
				N: "client",
				NextPath: &pathtest.Path[TransformContext]{
					N: "auth",
					NextPath: &pathtest.Path[TransformContext]{
						N: "attributes",
					},
				},
			},
			orig:         pcommon.NewMap(),
			newVal:       nil,
			wantErrOnSet: true,
		},
		{
			name: `client.auth.attributes["key"]`,
			path: &pathtest.Path[TransformContext]{
				N: "client",
				NextPath: &pathtest.Path[TransformContext]{
					N: "auth",
					NextPath: &pathtest.Path[TransformContext]{
						N: "attributes",
						KeySlice: []ottl.Key[TransformContext]{
							&pathtest.Key[TransformContext]{
								S: ottltest.Strp("key"),
							},
						},
					},
				},
			},
			orig:         nil,
			newVal:       nil,
			wantErrOnSet: true,
		},
		{
			name: `grpc.metadata`,
			path: &pathtest.Path[TransformContext]{
				N: "grpc",
				NextPath: &pathtest.Path[TransformContext]{
					N: "metadata",
				},
			},
			orig:         pcommon.NewMap(),
			newVal:       nil,
			wantErrOnSet: true,
		},
		{
			name: `grpc.metadata["key"]`,
			path: &pathtest.Path[TransformContext]{
				N: "grpc",
				NextPath: &pathtest.Path[TransformContext]{
					N: "metadata",
					KeySlice: []ottl.Key[TransformContext]{
						&pathtest.Key[TransformContext]{
							S: ottltest.Strp("key"),
						},
					},
				},
			},
			orig:         nil,
			newVal:       nil,
			wantErrOnSet: true,
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ContextName
		testWithContext.path = ottl.Path[TransformContext](&pathWithContext)
		tests = append(tests, testWithContext)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheGetter := func(tCtx TransformContext) pcommon.Map {
				return tCtx.cache
			}
			accessor, err := pathExpressionParser(cacheGetter)(tt.path)
			assert.NoError(t, err)

			tCtx := NewTransformContext()
			got, err := accessor.Get(t.Context(), tCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(t.Context(), tCtx, tt.newVal)
			if tt.wantErrOnSet {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
