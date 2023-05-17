// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestTransformerConfigMissingBase(t *testing.T) {
	cfg := NewTransformerConfig("test", "")
	cfg.OutputIDs = []string{"test-output"}
	_, err := cfg.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required `type` field.")
}

func TestTransformerConfigMissingOutput(t *testing.T) {
	cfg := NewTransformerConfig("test", "test")
	_, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
}

func TestTransformerConfigValid(t *testing.T) {
	cfg := NewTransformerConfig("test", "test")
	cfg.OutputIDs = []string{"test-output"}
	_, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
}

func TestTransformerOnErrorDefault(t *testing.T) {
	cfg := NewTransformerConfig("test-id", "test-type")
	transformer, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.Equal(t, SendOnError, transformer.OnError)
}

func TestTransformerOnErrorInvalid(t *testing.T) {
	cfg := NewTransformerConfig("test", "test")
	cfg.OnError = "invalid"
	_, err := cfg.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "operator config has an invalid `on_error` field.")
}

func TestTransformerOperatorCanProcess(t *testing.T) {
	cfg := NewTransformerConfig("test", "test")
	transformer, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.True(t, transformer.CanProcess())
}

func TestTransformerDropOnError(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("Process", mock.Anything, mock.Anything).Return(nil)
	transformer := TransformerOperator{
		OnError: DropOnError,
		WriterOperator: WriterOperator{
			BasicOperator: BasicOperator{
				OperatorID:    "test-id",
				OperatorType:  "test-type",
				SugaredLogger: testutil.Logger(t),
			},
			OutputOperators: []operator.Operator{output},
			OutputIDs:       []string{"test-output"},
		},
	}
	ctx := context.Background()
	testEntry := entry.New()
	transform := func(e *entry.Entry) error {
		return fmt.Errorf("Failure")
	}

	err := transformer.ProcessWith(ctx, testEntry, transform)
	require.Error(t, err)
	output.AssertNotCalled(t, "Process", mock.Anything, mock.Anything)
}

func TestTransformerSendOnError(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("Process", mock.Anything, mock.Anything).Return(nil)
	transformer := TransformerOperator{
		OnError: SendOnError,
		WriterOperator: WriterOperator{
			BasicOperator: BasicOperator{
				OperatorID:    "test-id",
				OperatorType:  "test-type",
				SugaredLogger: testutil.Logger(t),
			},
			OutputOperators: []operator.Operator{output},
			OutputIDs:       []string{"test-output"},
		},
	}
	ctx := context.Background()
	testEntry := entry.New()
	transform := func(e *entry.Entry) error {
		return fmt.Errorf("Failure")
	}

	err := transformer.ProcessWith(ctx, testEntry, transform)
	require.Error(t, err)
	output.AssertCalled(t, "Process", mock.Anything, mock.Anything)
}

func TestTransformerProcessWithValid(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("Process", mock.Anything, mock.Anything).Return(nil)
	transformer := TransformerOperator{
		OnError: SendOnError,
		WriterOperator: WriterOperator{
			BasicOperator: BasicOperator{
				OperatorID:    "test-id",
				OperatorType:  "test-type",
				SugaredLogger: testutil.Logger(t),
			},
			OutputOperators: []operator.Operator{output},
			OutputIDs:       []string{"test-output"},
		},
	}
	ctx := context.Background()
	testEntry := entry.New()
	transform := func(e *entry.Entry) error {
		return nil
	}

	err := transformer.ProcessWith(ctx, testEntry, transform)
	require.NoError(t, err)
	output.AssertCalled(t, "Process", mock.Anything, mock.Anything)
}

func TestTransformerIf(t *testing.T) {
	cases := []struct {
		name        string
		ifExpr      string
		inputBody   string
		expected    string
		errExpected bool
	}{
		{
			name:      "NoIf",
			ifExpr:    "",
			inputBody: "test",
			expected:  "parsed",
		},
		{
			name:      "TrueIf",
			ifExpr:    "true",
			inputBody: "test",
			expected:  "parsed",
		},
		{
			name:      "FalseIf",
			ifExpr:    "false",
			inputBody: "test",
			expected:  "test",
		},
		{
			name:      "EvaluatedTrue",
			ifExpr:    "body == 'test'",
			inputBody: "test",
			expected:  "parsed",
		},
		{
			name:      "EvaluatedFalse",
			ifExpr:    "body == 'notest'",
			inputBody: "test",
			expected:  "test",
		},
		{
			name:        "FailingExpressionEvaluation",
			ifExpr:      "body.test.noexist == 'notest'",
			inputBody:   "test",
			expected:    "test",
			errExpected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewTransformerConfig("test", "test")
			cfg.IfExpr = tc.ifExpr

			transformer, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			transformer.OutputOperators = []operator.Operator{fake}

			e := entry.New()
			e.Body = tc.inputBody
			err = transformer.ProcessWith(context.Background(), e, func(e *entry.Entry) error {
				e.Body = "parsed"
				return nil
			})
			if tc.errExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			fake.ExpectBody(t, tc.expected)
		})
	}

	t.Run("InvalidIfExpr", func(t *testing.T) {
		cfg := NewTransformerConfig("test", "test")
		cfg.IfExpr = "'nonbool'"
		_, err := cfg.Build(testutil.Logger(t))
		require.Error(t, err)
	})
}
