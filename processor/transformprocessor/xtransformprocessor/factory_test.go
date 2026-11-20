// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xtransformprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

func createTestFuncFactory[K any](name string) ottl.Factory[K] {
	type TestFuncArguments[K any] struct{}
	createFunc := func(_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[K], error) {
		return func(_ context.Context, _ K) (any, error) {
			return nil, nil
		}, nil
	}
	return ottl.NewFactory(name, &TestFuncArguments[K]{}, createFunc)
}

func Test_FactoryWithFunctions_CreateProfiles(t *testing.T) {
	type testCase struct {
		name           string
		statements     []transformprocessor.ContextStatements
		factoryOptions []transformprocessor.FactoryOption
		wantErrorWith  string
	}

	tests := []testCase{
		{
			name: "with resource functions : statement with added resource func",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("resource"),
					Statements: []string{`set(attributes["attr"], TestResourceFunc())`},
				},
			},
			factoryOptions: []transformprocessor.FactoryOption{
				transformprocessor.WithResourceFunctions(transformprocessor.DefaultResourceFunctions()),
				transformprocessor.WithResourceFunctions([]ottl.Factory[*ottlresource.TransformContext]{createTestFuncFactory[*ottlresource.TransformContext]("TestResourceFunc")}),
			},
		},
		{
			name: "with resource functions : statement with missing resource func",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("resource"),
					Statements: []string{`set(attributes["attr"], TestResourceFunc())`},
				},
			},
			wantErrorWith: `undefined function "TestResourceFunc"`,
			factoryOptions: []transformprocessor.FactoryOption{
				transformprocessor.WithResourceFunctions(transformprocessor.DefaultResourceFunctions()),
			},
		},
		{
			name: "with scope functions : statement with added scope func",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("scope"),
					Statements: []string{`set(attributes["attr"], TestScopeFunc())`},
				},
			},
			factoryOptions: []transformprocessor.FactoryOption{
				transformprocessor.WithScopeFunctions(transformprocessor.DefaultScopeFunctions()),
				transformprocessor.WithScopeFunctions([]ottl.Factory[*ottlscope.TransformContext]{createTestFuncFactory[*ottlscope.TransformContext]("TestScopeFunc")}),
			},
		},
		{
			name: "with profile functions : statement with added profile func",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextIDProfile,
					Statements: []string{`set(cache["attr"], TestProfileFunc())`},
				},
			},
			factoryOptions: []transformprocessor.FactoryOption{
				WithProfileFunctions(DefaultProfileFunctions()),
				WithProfileFunctions([]ottl.Factory[*ottlprofile.TransformContext]{createTestFuncFactory[*ottlprofile.TransformContext]("TestProfileFunc")}),
			},
		},
		{
			name: "with profile functions : statement with missing profile func",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextIDProfile,
					Statements: []string{`set(cache["attr"], TestProfileFunc())`},
				},
			},
			wantErrorWith: `undefined function "TestProfileFunc"`,
			factoryOptions: []transformprocessor.FactoryOption{
				WithProfileFunctions(DefaultProfileFunctions()),
			},
		},
		{
			name: "with profile functions : only custom functions",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextIDProfile,
					Statements: []string{`testProfileFunc()`},
				},
			},
			factoryOptions: []transformprocessor.FactoryOption{
				WithProfileFunctions([]ottl.Factory[*ottlprofile.TransformContext]{createTestFuncFactory[*ottlprofile.TransformContext]("testProfileFunc")}),
			},
		},
		{
			name: "with profile functions : missing default functions",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextIDProfile,
					Statements: []string{`set(attributes["test"], TestProfileFunc())`},
				},
			},
			wantErrorWith: `undefined function "set"`,
			factoryOptions: []transformprocessor.FactoryOption{
				WithProfileFunctions([]ottl.Factory[*ottlprofile.TransformContext]{createTestFuncFactory[*ottlprofile.TransformContext]("TestProfileFunc")}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := transformprocessor.NewFactoryWithOptions(tt.factoryOptions...)
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*transformprocessor.Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.ProfileStatements = tt.statements

			validateErr := oCfg.Validate()
			if tt.wantErrorWith != "" {
				assert.ErrorContains(t, validateErr, tt.wantErrorWith)
			} else {
				assert.NoError(t, validateErr)
			}

			_, err := factory.(xprocessor.Factory).CreateProfiles(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			require.NoError(t, err)
		})
	}
}
