// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xtransformprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor"
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
				WithResourceFunctions(DefaultResourceFunctions()),
				WithResourceFunctions([]ottl.Factory[*ottlresource.TransformContext]{createTestFuncFactory[*ottlresource.TransformContext]("TestResourceFunc")}),
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
				WithResourceFunctions(DefaultResourceFunctions()),
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
				WithScopeFunctions(DefaultScopeFunctions()),
				WithScopeFunctions([]ottl.Factory[*ottlscope.TransformContext]{createTestFuncFactory[*ottlscope.TransformContext]("TestScopeFunc")}),
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

func Test_FactoryWithFunctions_ResourceAndScope(t *testing.T) {
	testResourceFunc := []ottl.Factory[*ottlresource.TransformContext]{createTestFuncFactory[*ottlresource.TransformContext]("TestResourceFunc")}
	testScopeFunc := []ottl.Factory[*ottlscope.TransformContext]{createTestFuncFactory[*ottlscope.TransformContext]("TestScopeFunc")}

	tests := []struct {
		name           string
		statements     []transformprocessor.ContextStatements
		factoryOptions []transformprocessor.FactoryOption
		wantErrorWith  string
	}{
		{
			name: "with resource functions : statement with added resource func",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("resource"),
					Statements: []string{`set(attributes["attr"], TestResourceFunc())`},
				},
			},
			factoryOptions: []transformprocessor.FactoryOption{
				WithResourceFunctions(DefaultResourceFunctions()),
				WithResourceFunctions(testResourceFunc),
			},
		},
		{
			name: "with resource functions : statement with inferred resource context",
			statements: []transformprocessor.ContextStatements{
				{
					Statements: []string{`set(resource.attributes["attr"], TestResourceFunc())`},
				},
			},
			factoryOptions: []transformprocessor.FactoryOption{
				WithResourceFunctions(DefaultResourceFunctions()),
				WithResourceFunctions(testResourceFunc),
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
				WithResourceFunctions(DefaultResourceFunctions()),
			},
		},
		{
			name: "with resource functions : only custom functions",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("resource"),
					Statements: []string{`testResourceFunc()`},
				},
			},
			factoryOptions: []transformprocessor.FactoryOption{
				WithResourceFunctions([]ottl.Factory[*ottlresource.TransformContext]{createTestFuncFactory[*ottlresource.TransformContext]("testResourceFunc")}),
			},
		},
		{
			name: "with resource functions : missing default functions",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("resource"),
					Statements: []string{`set(attributes["test"], "TestResourceFunc()")`},
				},
			},
			wantErrorWith: `undefined function "set"`,
			factoryOptions: []transformprocessor.FactoryOption{
				WithResourceFunctions(testResourceFunc),
			},
		},
		{
			name: "with resource functions : scope context keeps its default functions",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("scope"),
					Statements: []string{`set(attributes["attr"], "value")`},
				},
			},
			factoryOptions: []transformprocessor.FactoryOption{
				WithResourceFunctions(testResourceFunc),
			},
		},
		{
			name: "with resource functions : resource func is not available in the scope context",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("scope"),
					Statements: []string{`set(attributes["attr"], TestResourceFunc())`},
				},
			},
			wantErrorWith: `undefined function "TestResourceFunc"`,
			factoryOptions: []transformprocessor.FactoryOption{
				WithResourceFunctions(DefaultResourceFunctions()),
				WithResourceFunctions(testResourceFunc),
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
				WithScopeFunctions(DefaultScopeFunctions()),
				WithScopeFunctions(testScopeFunc),
			},
		},
		{
			name: "with scope functions : statement with inferred scope context",
			statements: []transformprocessor.ContextStatements{
				{
					Statements: []string{`set(scope.attributes["attr"], TestScopeFunc())`},
				},
			},
			factoryOptions: []transformprocessor.FactoryOption{
				WithScopeFunctions(DefaultScopeFunctions()),
				WithScopeFunctions(testScopeFunc),
			},
		},
		{
			name: "with scope functions : statement with missing scope func",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("scope"),
					Statements: []string{`set(attributes["attr"], TestScopeFunc())`},
				},
			},
			wantErrorWith: `undefined function "TestScopeFunc"`,
			factoryOptions: []transformprocessor.FactoryOption{
				WithScopeFunctions(DefaultScopeFunctions()),
			},
		},
		{
			name: "with scope functions : only custom functions",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("scope"),
					Statements: []string{`testScopeFunc()`},
				},
			},
			factoryOptions: []transformprocessor.FactoryOption{
				WithScopeFunctions([]ottl.Factory[*ottlscope.TransformContext]{createTestFuncFactory[*ottlscope.TransformContext]("testScopeFunc")}),
			},
		},
		{
			name: "with scope functions : missing default functions",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("scope"),
					Statements: []string{`set(attributes["test"], "TestScopeFunc()")`},
				},
			},
			wantErrorWith: `undefined function "set"`,
			factoryOptions: []transformprocessor.FactoryOption{
				WithScopeFunctions(testScopeFunc),
			},
		},
		{
			name: "with scope functions : resource context keeps its default functions",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("resource"),
					Statements: []string{`set(attributes["attr"], "value")`},
				},
			},
			factoryOptions: []transformprocessor.FactoryOption{
				WithScopeFunctions(testScopeFunc),
			},
		},
		{
			name: "with resource functions : empty list removes the default functions",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("resource"),
					Statements: []string{`set(attributes["attr"], "value")`},
				},
			},
			wantErrorWith: `undefined function "set"`,
			factoryOptions: []transformprocessor.FactoryOption{
				WithResourceFunctions(nil),
			},
		},
		{
			name: "with scope functions : empty list removes the default functions",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("scope"),
					Statements: []string{`set(attributes["attr"], "value")`},
				},
			},
			wantErrorWith: `undefined function "set"`,
			factoryOptions: []transformprocessor.FactoryOption{
				WithScopeFunctions(nil),
			},
		},
		{
			name: "without options : resource and scope contexts use the default functions",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("resource"),
					Statements: []string{`set(attributes["attr"], "value")`},
				},
				{
					Context:    transformprocessor.ContextID("scope"),
					Statements: []string{`set(attributes["attr"], "value")`},
				},
			},
		},
		{
			name: "without options : custom resource func is undefined",
			statements: []transformprocessor.ContextStatements{
				{
					Context:    transformprocessor.ContextID("resource"),
					Statements: []string{`set(attributes["attr"], TestResourceFunc())`},
				},
			},
			wantErrorWith: `undefined function "TestResourceFunc"`,
		},
	}

	signals := []struct {
		name          string
		setStatements func(cfg *transformprocessor.Config, statements []transformprocessor.ContextStatements)
		create        func(t *testing.T, factory processor.Factory, cfg component.Config) error
	}{
		{
			name: "traces",
			setStatements: func(cfg *transformprocessor.Config, statements []transformprocessor.ContextStatements) {
				cfg.TraceStatements = statements
			},
			create: func(t *testing.T, factory processor.Factory, cfg component.Config) error {
				_, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
				return err
			},
		},
		{
			name: "metrics",
			setStatements: func(cfg *transformprocessor.Config, statements []transformprocessor.ContextStatements) {
				cfg.MetricStatements = statements
			},
			create: func(t *testing.T, factory processor.Factory, cfg component.Config) error {
				_, err := factory.CreateMetrics(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
				return err
			},
		},
		{
			name: "logs",
			setStatements: func(cfg *transformprocessor.Config, statements []transformprocessor.ContextStatements) {
				cfg.LogStatements = statements
			},
			create: func(t *testing.T, factory processor.Factory, cfg component.Config) error {
				_, err := factory.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
				return err
			},
		},
	}

	for _, tt := range tests {
		for _, signal := range signals {
			t.Run(tt.name+" : "+signal.name, func(t *testing.T) {
				factory := transformprocessor.NewFactoryWithOptions(tt.factoryOptions...)
				cfg := factory.CreateDefaultConfig()
				oCfg := cfg.(*transformprocessor.Config)
				oCfg.ErrorMode = ottl.IgnoreError
				signal.setStatements(oCfg, tt.statements)

				validateErr := oCfg.Validate()
				createErr := signal.create(t, factory, cfg)
				if tt.wantErrorWith != "" {
					require.ErrorContains(t, validateErr, tt.wantErrorWith)
					require.ErrorContains(t, createErr, tt.wantErrorWith)
					return
				}
				require.NoError(t, validateErr)
				require.NoError(t, createErr)
			})
		}
	}
}

func Test_DefaultResourceAndScopeFunctions(t *testing.T) {
	resourceFunctions := map[string]bool{}
	for _, f := range DefaultResourceFunctions() {
		resourceFunctions[f.Name()] = true
	}
	assert.True(t, resourceFunctions["set"], "default resource functions must contain the standard OTTL functions")

	scopeFunctions := map[string]bool{}
	for _, f := range DefaultScopeFunctions() {
		scopeFunctions[f.Name()] = true
	}
	assert.True(t, scopeFunctions["set"], "default scope functions must contain the standard OTTL functions")
}
