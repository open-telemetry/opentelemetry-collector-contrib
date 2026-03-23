// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

func newTestTransformContext(lr plog.LogRecord, resource pcommon.Resource, scope pcommon.InstrumentationScope) *ottllog.TransformContext {
	sl := plog.NewScopeLogs()
	scope.CopyTo(sl.Scope())
	rl := plog.NewResourceLogs()
	resource.CopyTo(rl.Resource())
	return ottllog.NewTransformContextPtr(rl, sl, lr)
}

func TestNewOTTLLogRecordExpression(t *testing.T) {
	set := component.TelemetrySettings{Logger: zap.NewNop()}

	t.Run("valid expression accessing body", func(t *testing.T) {
		expr, err := NewOTTLLogRecordExpression("body", set)
		require.NoError(t, err)
		require.NotNil(t, expr)

		lr := plog.NewLogRecord()
		lr.Body().SetStr("hello world")

		tCtx := newTestTransformContext(lr, pcommon.NewResource(), pcommon.NewInstrumentationScope())
		val, err := expr.Execute(t.Context(), tCtx)
		require.NoError(t, err)
		require.Equal(t, "hello world", val)
	})

	t.Run("valid expression accessing resource attribute", func(t *testing.T) {
		expr, err := NewOTTLLogRecordExpression(`resource.attributes["service.name"]`, set)
		require.NoError(t, err)
		require.NotNil(t, expr)

		lr := plog.NewLogRecord()
		res := pcommon.NewResource()
		res.Attributes().PutStr("service.name", "my-service")

		tCtx := newTestTransformContext(lr, res, pcommon.NewInstrumentationScope())
		val, err := expr.Execute(t.Context(), tCtx)
		require.NoError(t, err)
		require.Equal(t, "my-service", val)
	})

	t.Run("valid expression accessing log attribute", func(t *testing.T) {
		expr, err := NewOTTLLogRecordExpression(`attributes["key"]`, set)
		require.NoError(t, err)
		require.NotNil(t, expr)

		lr := plog.NewLogRecord()
		lr.Attributes().PutStr("key", "value")

		tCtx := newTestTransformContext(lr, pcommon.NewResource(), pcommon.NewInstrumentationScope())
		val, err := expr.Execute(t.Context(), tCtx)
		require.NoError(t, err)
		require.Equal(t, "value", val)
	})

	t.Run("valid expression with converter function", func(t *testing.T) {
		expr, err := NewOTTLLogRecordExpression(`Concat([attributes["a"], attributes["b"]], "-")`, set)
		require.NoError(t, err)
		require.NotNil(t, expr)

		lr := plog.NewLogRecord()
		lr.Attributes().PutStr("a", "foo")
		lr.Attributes().PutStr("b", "bar")

		tCtx := newTestTransformContext(lr, pcommon.NewResource(), pcommon.NewInstrumentationScope())
		val, err := expr.Execute(t.Context(), tCtx)
		require.NoError(t, err)
		require.Equal(t, "foo-bar", val)
	})

	t.Run("invalid expression", func(t *testing.T) {
		expr, err := NewOTTLLogRecordExpression("not_a_valid_expr!!!", set)
		require.Error(t, err)
		require.Nil(t, expr)
	})
}

func TestNewOTTLLogRecordStatement(t *testing.T) {
	set := component.TelemetrySettings{Logger: zap.NewNop()}

	t.Run("valid statement", func(t *testing.T) {
		stmt, err := NewOTTLLogRecordStatement("value(body)", set)
		require.NoError(t, err)
		require.NotNil(t, stmt)
	})

	t.Run("invalid statement", func(t *testing.T) {
		stmt, err := NewOTTLLogRecordStatement("invalid!!!", set)
		require.Error(t, err)
		require.Nil(t, stmt)
	})
}

func TestCreateValueFunction_InvalidArgs(t *testing.T) {
	_, err := createValueFunction(ottl.FunctionContext{}, "not_valid_args")
	require.Error(t, err)
	require.Contains(t, err.Error(), "valueFactory args must be of type *valueArguments")
}
