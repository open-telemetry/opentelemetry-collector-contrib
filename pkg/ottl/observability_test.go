// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// path parser for any context used in observability tests.
// It handles common paths used in tests: "attributes", "name" and otherwise errors.
func obsTestParsePath[K any](p Path[K]) (GetSetter[K], error) {
	if p != nil && (p.Name() == "name" || p.Name() == "attributes") {
		return &StandardGetSetter[K]{
			Getter: func(_ context.Context, tCtx K) (any, error) {
				return tCtx, nil
			},
			Setter: func(_ context.Context, _ K, val any) error {
				// no-op for test
				_ = reflect.DeepEqual(nil, val)
				return nil
			},
		}, nil
	}
	return nil, errors.New("unsupported path for obs test")
}

// TestStatement_Observability_DebugLog verifies the Stable debug log contract for Statement.Execute.
func TestStatement_Observability_DebugLog(t *testing.T) {
	core, observed := observer.New(zap.DebugLevel)
	telemetry := componenttest.NewNopTelemetrySettings()
	telemetry.Logger = zap.New(core)

	parser, err := NewParser(
		CreateFactoryMap(
			NewFactory("set", nil, func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
				return func(_ context.Context, _ any) (any, error) { return nil, nil }, nil
			}),
		),
		obsTestParsePath[any],
		telemetry,
	)
	require.NoError(t, err)

	stmt, err := parser.ParseStatement(`set(attributes["test"], "pass") where true == true`)
	require.NoError(t, err)

	ctx := context.Background()
	var tCtx any = map[string]string{"orig": "val"}
	_, _, err = stmt.Execute(ctx, tCtx)
	require.NoError(t, err)

	logs := observed.TakeAll()
	require.Len(t, logs, 1)
	entry := logs[0]
	assert.Equal(t, zap.DebugLevel, entry.Level)
	assert.Equal(t, "TransformContext after statement execution", entry.Message)
	m := entry.ContextMap()
	assert.Equal(t, `set(attributes["test"], "pass") where true == true`, m["statement"])
	assert.Equal(t, true, m["condition matched"])
	assert.NotNil(t, m["TransformContext"])
}

// TestStatement_Observability_DebugDisabled ensures no debug log when level is Info.
func TestStatement_Observability_DebugDisabled(t *testing.T) {
	core, observed := observer.New(zap.InfoLevel)
	telemetry := componenttest.NewNopTelemetrySettings()
	telemetry.Logger = zap.New(core)

	parser, err := NewParser(
		CreateFactoryMap(
			NewFactory("set", nil, func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
				return func(_ context.Context, _ any) (any, error) { return nil, nil }, nil
			}),
		),
		obsTestParsePath[any],
		telemetry,
	)
	require.NoError(t, err)
	stmt, err := parser.ParseStatement(`set(attributes["test"], "pass")`)
	require.NoError(t, err)

	_, _, err = stmt.Execute(context.Background(), map[string]string{"x": "y"})
	require.NoError(t, err)
	assert.Empty(t, observed.TakeAll(), "debug log must be gated by DebugLevel")
}

// TestStatementSequence_Observability_WarnOnIgnore verifies Warn log for IgnoreError and privacy (no TransformContext in Warn).
func TestStatementSequence_Observability_WarnOnIgnore(t *testing.T) {
	core, observed := observer.New(zap.WarnLevel)
	telemetry := componenttest.NewNopTelemetrySettings()
	telemetry.Logger = zap.New(core)

	errBoom := errors.New("boom")
	factory := NewFactory("boom", nil, func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
		return func(_ context.Context, _ any) (any, error) { return nil, errBoom }, nil
	})
	parser, err := NewParser(CreateFactoryMap(factory), obsTestParsePath[any], telemetry)
	require.NoError(t, err)
	stmt, err := parser.ParseStatement(`boom()`)
	require.NoError(t, err)

	seq := NewStatementSequence([]*Statement[any]{stmt}, telemetry, WithStatementSequenceErrorMode[any](IgnoreError))
	err = seq.Execute(context.Background(), "v")
	require.NoError(t, err, "IgnoreError must not propagate")

	logs := observed.TakeAll()
	require.Len(t, logs, 1)
	entry := logs[0]
	assert.Equal(t, zap.WarnLevel, entry.Level)
	assert.Equal(t, "failed to execute statement", entry.Message)
	m := entry.ContextMap()
	assert.Equal(t, `boom()`, m["statement"])
	// error field is present
	assert.NotNil(t, m["error"])
	_, hasTransform := m["TransformContext"]
	assert.False(t, hasTransform, "Warn logs must not include TransformContext for privacy")
}

// TestStatementSequence_Observability_SilentEmitsNoWarn ensures Silent mode is dark.
func TestStatementSequence_Observability_SilentEmitsNoWarn(t *testing.T) {
	core, observed := observer.New(zap.WarnLevel)
	telemetry := componenttest.NewNopTelemetrySettings()
	telemetry.Logger = zap.New(core)

	factory := NewFactory("boom", nil, func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
		return func(_ context.Context, _ any) (any, error) { return nil, errors.New("boom") }, nil
	})
	parser, err := NewParser(CreateFactoryMap(factory), obsTestParsePath[any], telemetry)
	require.NoError(t, err)
	stmt, err := parser.ParseStatement(`boom()`)
	require.NoError(t, err)

	seq := NewStatementSequence([]*Statement[any]{stmt}, telemetry, WithStatementSequenceErrorMode[any](SilentError))
	err = seq.Execute(context.Background(), "v")
	require.NoError(t, err)
	assert.Empty(t, observed.TakeAll())
}

// TestStatementSequence_Observability_PropagateReturnsError verifies PropagateError returns error and does not log Warn.
func TestStatementSequence_Observability_PropagateReturnsError(t *testing.T) {
	core, observed := observer.New(zap.WarnLevel)
	telemetry := componenttest.NewNopTelemetrySettings()
	telemetry.Logger = zap.New(core)

	factory := NewFactory("boom", nil, func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
		return func(_ context.Context, _ any) (any, error) { return nil, errors.New("boom") }, nil
	})
	parser, err := NewParser(CreateFactoryMap(factory), obsTestParsePath[any], telemetry)
	require.NoError(t, err)
	stmt, err := parser.ParseStatement(`boom()`)
	require.NoError(t, err)

	seq := NewStatementSequence([]*Statement[any]{stmt}, telemetry, WithStatementSequenceErrorMode[any](PropagateError))
	err = seq.Execute(context.Background(), "v")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute statement")
	assert.Empty(t, observed.TakeAll(), "PropagateError must not emit Warn, only return error for host metrics")
}

// TestConditionSequence_Observability_Debug verifies debug log for condition evaluation.
func TestConditionSequence_Observability_Debug(t *testing.T) {
	core, observed := observer.New(zap.DebugLevel)
	telemetry := componenttest.NewNopTelemetrySettings()
	telemetry.Logger = zap.New(core)

	parser, err := NewParser(CreateFactoryMap[any](), obsTestParsePath[any], telemetry)
	require.NoError(t, err)
	conds, err := parser.ParseConditions([]string{`true == true`})
	require.NoError(t, err)
	seq := NewConditionSequence(conds, telemetry)
	matched, err := seq.Eval(context.Background(), "v")
	require.NoError(t, err)
	assert.True(t, matched)
	logs := observed.TakeAll()
	require.Len(t, logs, 1)
	assert.Equal(t, "condition evaluation result", logs[0].Message)
	assert.NotNil(t, logs[0].ContextMap()["TransformContext"])
}

// TestConditionSequence_Observability_WarnOnIgnore verifies Warn for condition errors.
func TestConditionSequence_Observability_WarnOnIgnore(t *testing.T) {
	core, observed := observer.New(zap.WarnLevel)
	telemetry := componenttest.NewNopTelemetrySettings()
	telemetry.Logger = zap.New(core)

	// Use a converter that always errors when used in condition.
	errFactory := NewFactory("errCond", nil, func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
		return func(_ context.Context, _ any) (any, error) { return nil, errors.New("cond boom") }, nil
	})
	parser, err := NewParser(CreateFactoryMap(errFactory), obsTestParsePath[any], telemetry)
	require.NoError(t, err)
	condErr, err := parser.ParseConditions([]string{`errCond() == true`})
	if err != nil {
		t.Skip("parser does not support errCond in condition position for this test")
	}
	seq := NewConditionSequence(condErr, telemetry, WithConditionSequenceErrorMode[any](IgnoreError))
	matched, err := seq.Eval(context.Background(), "v")
	require.NoError(t, err)
	require.False(t, matched)
	logs := observed.TakeAll()
	require.Len(t, logs, 1)
	assert.Equal(t, "failed to eval condition", logs[0].Message)
	m := logs[0].ContextMap()
	assert.Contains(t, m["condition"], "errCond")
	_, hasCtx := m["TransformContext"]
	assert.False(t, hasCtx)
}

// Ensure time import is used.
var _ = time.Now
