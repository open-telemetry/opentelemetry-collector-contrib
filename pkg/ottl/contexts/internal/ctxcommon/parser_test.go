// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxcommon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcache"
)

func TestNewParser(t *testing.T) {
	functions := map[string]ottl.Factory[testContext]{}
	pathExpressionParser := func(_ ottl.Path[testContext]) (ottl.GetSetter[testContext], error) {
		return &testGetSetter{value: "test"}, nil
	}
	enumParser := func(*ottl.EnumSymbol) (*ottl.Enum, error) {
		return nil, nil
	}

	parser, err := NewParser(
		functions,
		componenttest.NewNopTelemetrySettings(),
		pathExpressionParser,
		enumParser,
	)

	assert.NoError(t, err)
	assert.NotEqual(t, ottl.Parser[testContext]{}, parser)
}

func TestNewParserWithOptions(t *testing.T) {
	functions := map[string]ottl.Factory[testContext]{}
	pathExpressionParser := func(_ ottl.Path[testContext]) (ottl.GetSetter[testContext], error) {
		return &testGetSetter{value: "test"}, nil
	}
	enumParser := func(*ottl.EnumSymbol) (*ottl.Enum, error) {
		return nil, nil
	}

	customOption := func(_ *ottl.Parser[testContext]) {}

	parser, err := NewParser(
		functions,
		componenttest.NewNopTelemetrySettings(),
		pathExpressionParser,
		enumParser,
		customOption,
	)

	assert.NoError(t, err)
	assert.NotEqual(t, ottl.Parser[testContext]{}, parser)
}

func TestPathExpressionParser(t *testing.T) {
	const (
		contextName      = "test"
		contextDocRef    = "https://fake.com"
		otherContextName = "other"
	)

	cacheMap := pcommon.NewMap()
	cacheMap.PutStr("key", "cached-value")

	cacheGetter := func(testContext) pcommon.Map {
		return cacheMap
	}

	contextParsers := map[string]ottl.PathExpressionParser[testContext]{
		contextName: func(_ ottl.Path[testContext]) (ottl.GetSetter[testContext], error) {
			return &testGetSetter{value: "test-context-value"}, nil
		},
		otherContextName: func(_ ottl.Path[testContext]) (ottl.GetSetter[testContext], error) {
			return &testGetSetter{value: "other-context-value"}, nil
		},
	}

	parser := PathExpressionParser(
		contextName,
		contextDocRef,
		cacheGetter,
		contextParsers,
	)

	tests := []struct {
		name           string
		path           ottl.Path[testContext]
		skipValueCheck bool
		expectedType   string
		expectedValue  any
		wantErr        bool
		errContains    string
	}{
		{
			name:        "nil path returns error",
			path:        nil,
			wantErr:     true,
			errContains: "is not a valid path",
		},
		{
			name: "cache access from current context",
			path: &testPath{
				pathName: ctxcache.Name,
				nextPath: nil,
				pathStr:  ctxcache.Name,
			},
			expectedType:   "pcommon.Map",
			skipValueCheck: true, // Skip value check for map type
			wantErr:        false,
		},
		{
			name: "cache access from non-current context returns error",
			path: &testPath{
				ctx:      otherContextName,
				pathName: ctxcache.Name,
				pathStr:  otherContextName + "." + ctxcache.Name,
			},
			wantErr:     true,
			errContains: "access to cache must be performed using the same context",
		},
		{
			name: "valid path with current context",
			path: &testPath{
				pathName: "valid",
				pathStr:  "valid",
			},
			expectedType:  "string",
			expectedValue: "test-context-value",
			wantErr:       false,
		},
		{
			name: "valid path with explicit context",
			path: &testPath{
				ctx:      otherContextName,
				pathName: "valid",
				pathStr:  otherContextName + ".valid",
			},
			expectedType:  "string",
			expectedValue: "other-context-value",
			wantErr:       false,
		},
		{
			name: "invalid context returns error",
			path: &testPath{
				ctx:      "invalid",
				pathName: "invalid",
				pathStr:  "invalid.invalid",
			},
			wantErr:     true,
			errContains: "is not a valid path",
		},
		{
			name: "handles segment name matching a context",
			path: &testPath{
				pathName: otherContextName,
				pathStr:  otherContextName,
				nextPath: &testPath{
					pathName: "field",
					pathStr:  otherContextName + ".field",
				},
			},
			expectedType:  "string",
			expectedValue: "other-context-value",
			wantErr:       false,
		},
		{
			name: "segment name matching context but no next path returns error",
			path: &testPath{
				pathName: otherContextName,
				pathStr:  otherContextName,
				nextPath: nil,
			},
			wantErr:     true,
			errContains: "is not a valid path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getter, err := parser(tt.path)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, getter)

			if tt.skipValueCheck {
				return
			}

			val, err := getter.Get(context.Background(), testContext{})
			require.NoError(t, err)

			switch tt.expectedType {
			case "string":
				strVal, ok := val.(string)
				assert.True(t, ok, "Value should be a string but was %T", val)
				assert.Equal(t, tt.expectedValue, strVal)
			case "pcommon.Map":
				_, ok := val.(pcommon.Map)
				assert.True(t, ok, "Value should be a pcommon.Map but was %T", val)
			default:
				t.Fatalf("Unexpected expected type: %s", tt.expectedType)
			}
		})
	}
}

type testContext struct{}

type testPath struct {
	ctx      string
	pathName string
	nextPath ottl.Path[testContext]
	pathStr  string
}

func (p testPath) Context() string {
	return p.ctx
}

func (p testPath) Name() string {
	return p.pathName
}

func (p testPath) Next() ottl.Path[testContext] {
	return p.nextPath
}

func (p testPath) Keys() []ottl.Key[testContext] {
	return nil
}

func (p testPath) String() string {
	if p.pathStr != "" {
		return p.pathStr
	}
	return p.pathName
}

type testGetSetter struct {
	value any
}

func (m *testGetSetter) Get(_ context.Context, _ testContext) (any, error) {
	return m.value, nil
}

func (m *testGetSetter) Set(_ context.Context, _ testContext, _ any) error {
	return nil
}
