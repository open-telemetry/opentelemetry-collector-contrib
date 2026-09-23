// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

func Test_isInCIDR_parser_slice_arguments(t *testing.T) {
	parser, err := ottllog.NewParser(
		map[string]ottl.Factory[*ottllog.TransformContext]{
			"IsInCIDR": NewIsInCIDRFactory[*ottllog.TransformContext](),
			"Split":    NewSplitFactory[*ottllog.TransformContext](),
			"set":      NewSetFactory[*ottllog.TransformContext](),
		},
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)

	tests := []struct {
		name            string
		statement       string
		target          string
		setupAttributes func(pcommon.Map)
		setupCache      func(pcommon.Map)
		want            bool
		wantErrPart     string
	}{
		{
			name:      "cache slice matches",
			statement: `set(attributes["result"], IsInCIDR(attributes["client.address"], cache["networks"]))`,
			target:    "192.0.2.1",
			setupCache: func(cache pcommon.Map) {
				networks := cache.PutEmptySlice("networks")
				networks.AppendEmpty().SetStr("198.51.100.0/24")
				networks.AppendEmpty().SetStr("192.0.2.0/24")
			},
			want: true,
		},
		{
			name:      "Split result matches",
			statement: `set(attributes["result"], IsInCIDR(attributes["client.address"], Split(attributes["allowed_networks"], ",")))`,
			target:    "192.0.2.1",
			setupAttributes: func(attributes pcommon.Map) {
				attributes.PutStr("allowed_networks", "198.51.100.0/24,192.0.2.0/24")
			},
			want: true,
		},
		{
			name:        "unset cache entry",
			statement:   `set(attributes["result"], IsInCIDR(attributes["client.address"], cache["networks"]))`,
			target:      "192.0.2.1",
			wantErrPart: "networks cannot be nil",
		},
		{
			name:        "literal nil",
			statement:   `set(attributes["result"], IsInCIDR(attributes["client.address"], nil))`,
			target:      "192.0.2.1",
			wantErrPart: "networks cannot be nil",
		},
		{
			name:      "scalar cache value",
			statement: `set(attributes["result"], IsInCIDR(attributes["client.address"], cache["networks"]))`,
			target:    "192.0.2.1",
			setupCache: func(cache pcommon.Map) {
				cache.PutStr("networks", "192.0.2.0/24")
			},
			wantErrPart: "expected a slice",
		},
		{
			name:      "non-string after matching network",
			statement: `set(attributes["result"], IsInCIDR(attributes["client.address"], cache["networks"]))`,
			target:    "192.0.2.1",
			setupCache: func(cache pcommon.Map) {
				networks := cache.PutEmptySlice("networks")
				networks.AppendEmpty().SetStr("192.0.2.0/24")
				networks.AppendEmpty().SetInt(1)
			},
			wantErrPart: "expected string",
		},
		{
			name:      "empty cache slice",
			statement: `set(attributes["result"], IsInCIDR(attributes["client.address"], cache["networks"]))`,
			target:    "192.0.2.1",
			setupCache: func(cache pcommon.Map) {
				cache.PutEmptySlice("networks")
			},
			want: false,
		},
		{
			name:      "inline literal list",
			statement: `set(attributes["result"], IsInCIDR(attributes["client.address"], ["198.51.100.0/24", "192.0.2.0/24"]))`,
			target:    "192.0.2.1",
			want:      true,
		},
		{
			name:      "invalid target short-circuits invalid network source",
			statement: `set(attributes["result"], IsInCIDR(attributes["client.address"], cache["networks"]))`,
			target:    "not an IP address",
			setupCache: func(cache pcommon.Map) {
				cache.PutStr("networks", "not a slice")
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement, err := parser.ParseStatement(tt.statement)
			require.NoError(t, err)

			cache := pcommon.NewMap()
			if tt.setupCache != nil {
				tt.setupCache(cache)
			}

			resourceLogs := plog.NewResourceLogs()
			scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
			logRecord := scopeLogs.LogRecords().AppendEmpty()
			attributes := logRecord.Attributes()
			attributes.PutStr("client.address", tt.target)
			if tt.setupAttributes != nil {
				tt.setupAttributes(attributes)
			}

			tCtx := ottllog.NewTransformContext(resourceLogs, scopeLogs, logRecord, ottllog.WithCache(&cache))
			t.Cleanup(tCtx.Close)
			_, _, err = statement.Execute(t.Context(), tCtx)

			if tt.wantErrPart != "" {
				require.ErrorContains(t, err, tt.wantErrPart)
				return
			}
			require.NoError(t, err)
			result, ok := attributes.Get("result")
			require.True(t, ok)
			assert.Equal(t, tt.want, result.Bool())
		})
	}
}

func Test_isInCIDR(t *testing.T) {
	tests := []struct {
		name     string
		target   any
		networks []ottl.StringGetter[any]
		result   any
	}{
		{
			name:   "an included IP string",
			target: "192.0.2.1",
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.24.0/24", nil },
				},
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
				},
			},
			result: true,
		},
		{
			name:   "a not included IP string",
			target: "195.0.2.1",
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
				},
			},
			result: false,
		},
		{
			name:   "non IP string",
			target: "hello world",
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
				},
			},
			result: false,
		},
		{
			name:   "empty string",
			target: "",
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
				},
			},
			result: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := isInCIDR[any](ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) { return tt.target, nil },
			}, tt.networks)
			require.NoError(t, err)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.result, result)
		})
	}
}

func Test_isInCIDR_getter_errors(t *testing.T) {
	t.Run("target getter error", func(t *testing.T) {
		targetErr := errors.New("target getter failed")
		target := ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return nil, targetErr },
		}
		networks := []ottl.StringGetter[any]{
			ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
			},
		}

		exprFunc, err := isInCIDR[any](target, networks)
		require.NoError(t, err)

		_, err = exprFunc(t.Context(), nil)
		require.ErrorIs(t, err, targetErr)
	})

	t.Run("network getter error", func(t *testing.T) {
		networkErr := errors.New("network getter failed")
		target := ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.0.2.1", nil },
		}
		networks := []ottl.StringGetter[any]{
			ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) { return nil, networkErr },
			},
		}

		exprFunc, err := isInCIDR[any](target, networks)
		require.NoError(t, err)

		_, err = exprFunc(t.Context(), nil)
		require.ErrorIs(t, err, networkErr)
	})
}

func Test_isInCIDR_Error(t *testing.T) {
	tests := []struct {
		name          string
		target        any
		networks      []ottl.StringGetter[any]
		result        any
		err           bool
		expectedError string
	}{
		{
			name:   "non-string",
			target: 10,
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
				},
			},
			expectedError: "expected string but got int",
		},
		{
			name:   "dynamic network is not a valid CIDR",
			target: "192.0.0.1",
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2/24", nil },
				},
			},
			expectedError: "invalid CIDR address: 192.0.2/24",
		},
		{
			name:   "nil",
			target: nil,
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
				},
			},
			expectedError: "expected string but got nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := isInCIDR[any](ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) { return tt.target, nil },
			}, tt.networks)
			require.NoError(t, err)
			_, err = exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func Test_isInCIDR_literalNetworks(t *testing.T) {
	literalOne, err := ottl.NewTestingLiteralGetter(true, ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) { return "10.0.0.0/8", nil },
	})
	require.NoError(t, err)
	literalTwo, err := ottl.NewTestingLiteralGetter(true, ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) { return "192.168.0.0/16", nil },
	})
	require.NoError(t, err)

	t.Run("single literal network", func(t *testing.T) {
		exprFunc, err := isInCIDR[any](ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "10.1.2.3", nil },
		}, []ottl.StringGetter[any]{literalOne})
		require.NoError(t, err)
		result, err := exprFunc(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("multiple literals networks", func(t *testing.T) {
		exprFunc, err := isInCIDR[any](ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.168.0.1", nil },
		}, []ottl.StringGetter[any]{literalOne, literalTwo})
		require.NoError(t, err)
		result, err := exprFunc(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("invalid literal network", func(t *testing.T) {
		invalidLiteral, err := ottl.NewTestingLiteralGetter(true, ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.0.2/24", nil },
		})
		require.NoError(t, err)

		_, err = isInCIDR[any](ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.0.0.1", nil },
		}, []ottl.StringGetter[any]{invalidLiteral})
		assert.ErrorContains(t, err, "invalid CIDR address")
	})

	t.Run("invalid literal before dynamic getter", func(t *testing.T) {
		invalidLiteral, err := ottl.NewTestingLiteralGetter(true, ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.0.2/24", nil },
		})
		require.NoError(t, err)
		target := ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.0.2.1", nil },
		}
		dynamic := ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
		}
		networks := []ottl.StringGetter[any]{invalidLiteral, dynamic}

		_, err = isInCIDR[any](target, networks)
		assert.ErrorContains(t, err, "invalid CIDR address: 192.0.2/24")
	})
}

func Test_IsInCIDRFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewIsInCIDRFactory[any]()
		assert.Equal(t, "IsInCIDR", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewIsInCIDRFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &isInCIDRArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target", "Networks"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewIsInCIDRFactory[any]()
		args := factory.CreateDefaultArguments()
		isInCIDRArgs, ok := args.(*isInCIDRArguments[any])
		require.True(t, ok)
		isInCIDRArgs.Target = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "192.168.1.1", nil
			},
		}
		isInCIDRArgs.Networks = []ottl.StringGetter[any]{
			&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "192.168.1.0/24", nil
				},
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createIsInCIDRFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "IsInCIDRFactory args must be of type *isInCIDRArguments[K]")
	})
}

func BenchmarkIsInCIDR(b *testing.B) {
	exprFunc, err := isInCIDR[any](ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) { return "192.0.2.1", nil },
	}, []ottl.StringGetter[any]{
		ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
		},
	})
	require.NoError(b, err)
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := exprFunc(ctx, nil); err != nil {
			b.Fatal(err)
		}
	}
}
