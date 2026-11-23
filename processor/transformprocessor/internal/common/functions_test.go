// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

func TestResourceFunctions(t *testing.T) {
	funcs := ResourceFunctions()
	assert.NotEmpty(t, funcs)
	assert.Contains(t, funcs, "set")
}

func TestScopeFunctions(t *testing.T) {
	funcs := ScopeFunctions()
	assert.NotEmpty(t, funcs)
	assert.Contains(t, funcs, "set")
}

func TestNewExperimentalFactory(t *testing.T) {
	tests := []struct {
		name        string
		gateEnabled bool
		wantErr     string
	}{
		{
			name:        "gate disabled",
			gateEnabled: false,
			wantErr:     "function \"Experimental\" is experimental and requires the `processor.transform.enableExperimentalFunctions` feature gate to be enabled",
		},
		{
			name:        "gate enabled",
			gateEnabled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setExperimentalFunctionsFeatureGate(t, tt.gateEnabled)

			functions := ResourceFunctions()
			experimental := NewExperimentalFactory(ottl.NewFactory("Experimental", nil, func(ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[*ottlresource.TransformContext], error) {
				return func(context.Context, *ottlresource.TransformContext) (any, error) {
					return "value", nil
				}, nil
			}))
			functions[experimental.Name()] = experimental

			core, observedLogs := observer.New(zap.WarnLevel)
			settings := componenttest.NewNopTelemetrySettings()
			settings.Logger = zap.New(core)
			parser, err := ottlresource.NewParser(functions, settings)
			require.NoError(t, err)

			statement, err := parser.ParseStatement(`set(attributes["key"], Experimental())`)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Zero(t, observedLogs.Len())
				return
			}
			require.NoError(t, err)

			require.Equal(t, 1, observedLogs.Len())
			assert.Equal(t, "Experimental", observedLogs.All()[0].ContextMap()["function"])

			resource := pcommon.NewResource()
			_, _, err = statement.Execute(t.Context(), ottlresource.NewTransformContext(resource, pmetric.NewResourceMetrics()))
			require.NoError(t, err)
			value, ok := resource.Attributes().Get("key")
			require.True(t, ok)
			assert.Equal(t, "value", value.Str())
		})
	}
}

func setExperimentalFunctionsFeatureGate(t *testing.T, enabled bool) {
	gate := metadata.ProcessorTransformEnableExperimentalFunctionsFeatureGate
	original := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), original))
	})
}
