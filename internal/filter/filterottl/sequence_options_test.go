// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterottl

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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

type failureCounter struct {
	noop.Int64Counter

	count int64
}

func (c *failureCounter) Add(_ context.Context, incr int64, _ ...metric.AddOption) {
	c.count += incr
}

type failArguments struct{}

func failingFunctions() map[string]ottl.Factory[*ottllog.TransformContext] {
	return ottl.CreateFactoryMap(
		ottl.NewFactory("Fail", &failArguments{}, func(ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[*ottllog.TransformContext], error) {
			return func(context.Context, *ottllog.TransformContext) (any, error) { return nil, errors.New("test") }, nil
		}),
	)
}

func Test_NewBoolExprForLogWithOptions_SequenceOptions(t *testing.T) {
	counter := &failureCounter{}
	cs, err := NewBoolExprForLogWithOptions(
		[]string{"Fail()"},
		failingFunctions(),
		ottl.IgnoreError,
		componenttest.NewNopTelemetrySettings(),
		nil,
		ottllog.WithConditionSequenceTelemetry(ottl.SequenceTelemetry{FailureCounter: counter}),
	)
	require.NoError(t, err)

	_, err = cs.Eval(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), counter.count)
}

func Test_NewBoolExprForLogWithPathContextNames_SequenceOptions(t *testing.T) {
	counter := &failureCounter{}
	cs, err := NewBoolExprForLogWithPathContextNames(
		[]string{"Fail()"},
		failingFunctions(),
		ottl.IgnoreError,
		componenttest.NewNopTelemetrySettings(),
		ottllog.WithConditionSequenceTelemetry(ottl.SequenceTelemetry{FailureCounter: counter}),
	)
	require.NoError(t, err)

	_, err = cs.Eval(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), counter.count)
}
