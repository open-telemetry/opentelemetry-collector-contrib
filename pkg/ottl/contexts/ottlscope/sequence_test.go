// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlscope

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type failureCounter struct {
	noop.Int64Counter

	count int64
}

func (c *failureCounter) Add(_ context.Context, incr int64, _ ...metric.AddOption) {
	c.count += incr
}

type failArguments struct{}

func newFailingParser(t *testing.T) ottl.Parser[*TransformContext] {
	createFail := func(ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[*TransformContext], error) {
		return func(context.Context, *TransformContext) (any, error) { return nil, errors.New("test") }, nil
	}
	parser, err := NewParser(
		ottl.CreateFactoryMap(
			ottl.NewFactory("fail", &failArguments{}, createFail),
			ottl.NewFactory("Fail", &failArguments{}, createFail),
		),
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	return parser
}

func TestStatementSequenceTelemetry(t *testing.T) {
	parser := newFailingParser(t)
	statements, err := parser.ParseStatements([]string{"fail()"})
	require.NoError(t, err)

	counter := &failureCounter{}
	seq := NewStatementSequence(
		statements,
		componenttest.NewNopTelemetrySettings(),
		WithStatementSequenceErrorMode(ottl.IgnoreError),
		WithStatementSequenceTelemetry(ottl.SequenceTelemetry{FailureCounter: counter}),
	)
	require.NoError(t, seq.Execute(t.Context(), nil))
	assert.Equal(t, int64(1), counter.count)
}

func TestConditionSequenceTelemetry(t *testing.T) {
	parser := newFailingParser(t)
	conditions, err := parser.ParseConditions([]string{"Fail()"})
	require.NoError(t, err)

	counter := &failureCounter{}
	seq := NewConditionSequence(
		conditions,
		componenttest.NewNopTelemetrySettings(),
		WithConditionSequenceErrorMode(ottl.IgnoreError),
		WithConditionSequenceTelemetry(ottl.SequenceTelemetry{FailureCounter: counter}),
	)
	_, err = seq.Eval(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), counter.count)
}
