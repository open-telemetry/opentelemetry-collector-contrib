// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestTransformerConfigMissingBase(t *testing.T) {
	cfg := NewTransformerConfig("test", "")
	cfg.OutputIDs = []string{"test-output"}
	set := componenttest.NewNopTelemetrySettings()
	_, err := cfg.Build(set)
	require.ErrorContains(t, err, "missing required `type` field.")
}

func TestTransformerConfigMissingOutput(t *testing.T) {
	cfg := NewTransformerConfig("test", "test")
	set := componenttest.NewNopTelemetrySettings()
	_, err := cfg.Build(set)
	require.NoError(t, err)
}

func TestTransformerConfigValid(t *testing.T) {
	cfg := NewTransformerConfig("test", "test")
	cfg.OutputIDs = []string{"test-output"}
	set := componenttest.NewNopTelemetrySettings()
	_, err := cfg.Build(set)
	require.NoError(t, err)
}

func TestTransformerOnErrorDefault(t *testing.T) {
	cfg := NewTransformerConfig("test-id", "test-type")
	set := componenttest.NewNopTelemetrySettings()
	transformer, err := cfg.Build(set)
	require.NoError(t, err)
	require.Equal(t, SendOnError, transformer.OnError)
}

func TestTransformerOnErrorInvalid(t *testing.T) {
	cfg := NewTransformerConfig("test", "test")
	cfg.OnError = "invalid"
	set := componenttest.NewNopTelemetrySettings()
	_, err := cfg.Build(set)
	require.ErrorContains(t, err, "operator config has an invalid `on_error` field.")
}

func TestTransformerOperatorCanProcess(t *testing.T) {
	cfg := NewTransformerConfig("test", "test")
	set := componenttest.NewNopTelemetrySettings()
	transformer, err := cfg.Build(set)
	require.NoError(t, err)
	require.True(t, transformer.CanProcess())
}

func TestTransformerDropOnError(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("Process", mock.Anything, mock.Anything).Return(nil)

	obs, logs := observer.New(zap.WarnLevel)
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zap.New(obs)

	transformer := TransformerOperator{
		OnError: DropOnError,
		WriterOperator: WriterOperator{
			BasicOperator: BasicOperator{
				OperatorID:   "test-id",
				OperatorType: "test-type",
				set:          set,
			},
			OutputOperators: []operator.Operator{output},
			OutputIDs:       []string{"test-output"},
		},
	}
	ctx := context.Background()
	testEntry := entry.New()
	now := time.Now()
	testEntry.Timestamp = now
	testEntry.AddAttribute(attrs.LogFilePath, "/test/file")
	transform := func(_ *entry.Entry) error {
		return errors.New("Failure")
	}

	err := transformer.ProcessWith(ctx, testEntry, transform)
	require.Error(t, err)
	output.AssertNotCalled(t, "Process", mock.Anything, mock.Anything)

	// Test output logs
	expectedLogs := []observer.LoggedEntry{
		{
			Entry: zapcore.Entry{Level: zap.ErrorLevel, Message: "Failed to process entry"},
			Context: []zapcore.Field{
				{Key: "error", Type: zapcore.ErrorType, Interface: errors.New("Failure")},
				zap.Any("action", "drop"),
				zap.Any("entry.timestamp", now),
				zap.Any(attrs.LogFilePath, "/test/file"),
			},
		},
	}
	require.Equal(t, 1, logs.Len())
	require.Equalf(t, expectedLogs, logs.AllUntimed(), "expected logs do not match")
}

func TestTransformerDropOnErrorQuiet(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("Process", mock.Anything, mock.Anything).Return(nil)

	obs, logs := observer.New(zap.DebugLevel)
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zap.New(obs)

	transformer := TransformerOperator{
		OnError: DropOnErrorQuiet,
		WriterOperator: WriterOperator{
			BasicOperator: BasicOperator{
				OperatorID:   "test-id",
				OperatorType: "test-type",
				set:          set,
			},
			OutputOperators: []operator.Operator{output},
			OutputIDs:       []string{"test-output"},
		},
	}
	ctx := context.Background()
	testEntry := entry.New()
	now := time.Now()
	testEntry.Timestamp = now
	testEntry.AddAttribute(attrs.LogFilePath, "/test/file")
	transform := func(_ *entry.Entry) error {
		return errors.New("Failure")
	}

	err := transformer.ProcessWith(ctx, testEntry, transform)
	require.Error(t, err)
	output.AssertNotCalled(t, "Process", mock.Anything, mock.Anything)

	// Test output logs
	expectedLogs := []observer.LoggedEntry{
		{
			Entry: zapcore.Entry{Level: zap.DebugLevel, Message: "Failed to process entry"},
			Context: []zapcore.Field{
				{Key: "error", Type: 26, Interface: errors.New("Failure")},
				zap.Any("action", "drop_quiet"),
				zap.Any("entry.timestamp", now),
				zap.Any(attrs.LogFilePath, "/test/file"),
			},
		},
	}
	require.Equal(t, 1, logs.Len())
	require.Equalf(t, expectedLogs, logs.AllUntimed(), "expected logs do not match")
}

func TestTransformerSendOnError(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("Process", mock.Anything, mock.Anything).Return(nil)

	obs, logs := observer.New(zap.WarnLevel)
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zap.New(obs)

	transformer := TransformerOperator{
		OnError: SendOnError,
		WriterOperator: WriterOperator{
			BasicOperator: BasicOperator{
				OperatorID:   "test-id",
				OperatorType: "test-type",
				set:          set,
			},
			OutputOperators: []operator.Operator{output},
			OutputIDs:       []string{"test-output"},
		},
	}
	ctx := context.Background()
	testEntry := entry.New()
	now := time.Now()
	testEntry.Timestamp = now
	testEntry.AddAttribute(attrs.LogFilePath, "/test/file")
	transform := func(_ *entry.Entry) error {
		return errors.New("Failure")
	}

	err := transformer.ProcessWith(ctx, testEntry, transform)
	require.Error(t, err)
	output.AssertCalled(t, "Process", mock.Anything, mock.Anything)

	// Test output logs
	expectedLogs := []observer.LoggedEntry{
		{
			Entry: zapcore.Entry{Level: zap.ErrorLevel, Message: "Failed to process entry"},
			Context: []zapcore.Field{
				{Key: "error", Type: 26, Interface: errors.New("Failure")},
				zap.Any("action", "send"),
				zap.Any("entry.timestamp", now),
				zap.Any(attrs.LogFilePath, "/test/file"),
			},
		},
	}
	require.Equal(t, 1, logs.Len())
	require.Equalf(t, expectedLogs, logs.AllUntimed(), "expected logs do not match")
}

func TestTransformerSendOnErrorQuiet(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("Process", mock.Anything, mock.Anything).Return(nil)

	obs, logs := observer.New(zap.DebugLevel)
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zap.New(obs)

	transformer := TransformerOperator{
		OnError: SendOnErrorQuiet,
		WriterOperator: WriterOperator{
			BasicOperator: BasicOperator{
				OperatorID:   "test-id",
				OperatorType: "test-type",
				set:          set,
			},
			OutputOperators: []operator.Operator{output},
			OutputIDs:       []string{"test-output"},
		},
	}
	ctx := context.Background()
	testEntry := entry.New()
	now := time.Now()
	testEntry.Timestamp = now
	testEntry.AddAttribute(attrs.LogFilePath, "/test/file")
	transform := func(_ *entry.Entry) error {
		return errors.New("Failure")
	}

	err := transformer.ProcessWith(ctx, testEntry, transform)
	require.Error(t, err)
	output.AssertCalled(t, "Process", mock.Anything, mock.Anything)

	// Test output logs
	expectedLogs := []observer.LoggedEntry{
		{
			Entry: zapcore.Entry{Level: zap.DebugLevel, Message: "Failed to process entry"},
			Context: []zapcore.Field{
				{Key: "error", Type: 26, Interface: errors.New("Failure")},
				zap.Any("action", "send_quiet"),
				zap.Any("entry.timestamp", now),
				zap.Any(attrs.LogFilePath, "/test/file"),
			},
		},
	}
	require.Equal(t, 1, logs.Len())
	require.Equalf(t, expectedLogs, logs.AllUntimed(), "expected logs do not match")
}

func TestTransformerProcessWithValid(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("Process", mock.Anything, mock.Anything).Return(nil)
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	transformer := TransformerOperator{
		OnError: SendOnError,
		WriterOperator: WriterOperator{
			BasicOperator: BasicOperator{
				OperatorID:   "test-id",
				OperatorType: "test-type",
				set:          set,
			},
			OutputOperators: []operator.Operator{output},
			OutputIDs:       []string{"test-output"},
		},
	}
	ctx := context.Background()
	testEntry := entry.New()
	transform := func(_ *entry.Entry) error {
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

			set := componenttest.NewNopTelemetrySettings()
			transformer, err := cfg.Build(set)
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
		set := componenttest.NewNopTelemetrySettings()
		_, err := cfg.Build(set)
		require.Error(t, err)
	})
}
