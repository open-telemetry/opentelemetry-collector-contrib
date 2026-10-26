// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xottl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type noArgs struct{}

func newTestParser(t *testing.T) ottl.Parser[any] {
	funcs := ottl.CreateFactoryMap(
		ottl.NewFactory("fail", &noArgs{}, func(ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[any], error) {
			return func(context.Context, any) (any, error) { return nil, errors.New("test") }, nil
		}),
		ottl.NewFactory("pass", &noArgs{}, func(ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[any], error) {
			return func(context.Context, any) (any, error) { return nil, nil }, nil
		}),
		ottl.NewFactory("Fail", &noArgs{}, func(ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[any], error) {
			return func(context.Context, any) (any, error) { return nil, errors.New("test") }, nil
		}),
		ottl.NewFactory("Match", &noArgs{}, func(ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[any], error) {
			return func(context.Context, any) (any, error) { return true, nil }, nil
		}),
		ottl.NewFactory("NoMatch", &noArgs{}, func(ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[any], error) {
			return func(context.Context, any) (any, error) { return false, nil }, nil
		}),
	)
	p, err := ottl.NewParser(funcs, func(ottl.Path[any]) (ottl.GetSetter[any], error) {
		return nil, errors.New("paths are not supported")
	}, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	return p
}

// newTestFailureCounter returns a counter and a function returning the sum of everything recorded on it.
func newTestFailureCounter(t *testing.T) (metric.Int64Counter, func() int64) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { assert.NoError(t, mp.Shutdown(context.WithoutCancel(t.Context()))) })
	counter, err := mp.Meter("test").Int64Counter("failures")
	require.NoError(t, err)
	return counter, func() int64 {
		var rm metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(t.Context(), &rm))
		var total int64
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
					total += dp.Value
				}
			}
		}
		return total
	}
}

func TestWithStatementSequenceFailureCounter(t *testing.T) {
	tests := []struct {
		name       string
		statements []string
		errorMode  ottl.ErrorMode
		expected   int64
	}{
		{name: "ignore with failure", statements: []string{"pass()", "fail()"}, errorMode: ottl.IgnoreError, expected: 1},
		{name: "silent with failure", statements: []string{"fail()"}, errorMode: ottl.SilentError, expected: 1},
		{name: "ignore with several failures counts once", statements: []string{"fail()", "pass()", "fail()"}, errorMode: ottl.IgnoreError, expected: 1},
		{name: "ignore without failure", statements: []string{"pass()"}, errorMode: ottl.IgnoreError, expected: 0},
		{name: "propagate is not counted", statements: []string{"fail()"}, errorMode: ottl.PropagateError, expected: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestParser(t)
			statements, err := p.ParseStatements(tt.statements)
			require.NoError(t, err)

			counter, sum := newTestFailureCounter(t)
			seq := ottl.NewStatementSequence(
				statements,
				componenttest.NewNopTelemetrySettings(),
				ottl.WithStatementSequenceErrorMode[any](tt.errorMode),
				WithStatementSequenceFailureCounter[any](counter),
			)
			err = seq.Execute(t.Context(), nil)
			if tt.errorMode == ottl.PropagateError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expected, sum())
		})
	}

	t.Run("no counter set", func(t *testing.T) {
		p := newTestParser(t)
		statements, err := p.ParseStatements([]string{"fail()"})
		require.NoError(t, err)
		seq := ottl.NewStatementSequence(
			statements,
			componenttest.NewNopTelemetrySettings(),
			ottl.WithStatementSequenceErrorMode[any](ottl.IgnoreError),
		)
		require.NoError(t, seq.Execute(t.Context(), nil))
	})
}

func TestWithConditionSequenceFailureCounter(t *testing.T) {
	tests := []struct {
		name       string
		conditions []string
		errorMode  ottl.ErrorMode
		logicOp    ottl.LogicOperation
		expected   int64
	}{
		{name: "ignore with failure", conditions: []string{"Fail()"}, errorMode: ottl.IgnoreError, logicOp: ottl.Or, expected: 1},
		{name: "silent with failure", conditions: []string{"Fail()"}, errorMode: ottl.SilentError, logicOp: ottl.Or, expected: 1},
		{name: "several failures count once", conditions: []string{"Fail()", "Fail()"}, errorMode: ottl.IgnoreError, logicOp: ottl.Or, expected: 1},
		{name: "failure then early OR match", conditions: []string{"Fail()", "Match()"}, errorMode: ottl.IgnoreError, logicOp: ottl.Or, expected: 1},
		{name: "failure then AND completes", conditions: []string{"Fail()", "Match()"}, errorMode: ottl.IgnoreError, logicOp: ottl.And, expected: 1},
		{name: "failure then early AND false", conditions: []string{"Fail()", "NoMatch()"}, errorMode: ottl.IgnoreError, logicOp: ottl.And, expected: 1},
		{name: "no failure", conditions: []string{"Match()"}, errorMode: ottl.IgnoreError, logicOp: ottl.Or, expected: 0},
		{name: "propagate is not counted", conditions: []string{"Fail()"}, errorMode: ottl.PropagateError, logicOp: ottl.Or, expected: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestParser(t)
			conditions, err := p.ParseConditions(tt.conditions)
			require.NoError(t, err)

			counter, sum := newTestFailureCounter(t)
			seq := ottl.NewConditionSequence(
				conditions,
				componenttest.NewNopTelemetrySettings(),
				ottl.WithConditionSequenceErrorMode[any](tt.errorMode),
				ottl.WithLogicOperation[any](tt.logicOp),
				WithConditionSequenceFailureCounter[any](counter),
			)
			_, err = seq.Eval(t.Context(), nil)
			if tt.errorMode == ottl.PropagateError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expected, sum())
		})
	}

	t.Run("no counter set", func(t *testing.T) {
		p := newTestParser(t)
		conditions, err := p.ParseConditions([]string{"Fail()"})
		require.NoError(t, err)
		seq := ottl.NewConditionSequence(
			conditions,
			componenttest.NewNopTelemetrySettings(),
			ottl.WithConditionSequenceErrorMode[any](ottl.IgnoreError),
		)
		_, err = seq.Eval(t.Context(), nil)
		require.NoError(t, err)
	})
}
