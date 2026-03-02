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
	ctx := t.Context()
	testEntry := entry.New()
	now := time.Now()
	testEntry.Timestamp = now
	testEntry.AddAttribute(attrs.LogFilePath, "/test/file")
	transform := func(_ *entry.Entry) error {
		return errors.New("failure")
	}

	err := transformer.ProcessWith(ctx, testEntry, transform)
	require.Error(t, err)
	output.AssertNotCalled(t, "Process", mock.Anything, mock.Anything)

	// Test output logs
	expectedLogs := []observer.LoggedEntry{
		{
			Entry: zapcore.Entry{Level: zap.ErrorLevel, Message: "Failed to process entry"},
			Context: []zapcore.Field{
				zap.Error(errors.New("failure")),
				zap.String("action", "drop"),
				zap.Time("entry.timestamp", now),
				zap.String(attrs.LogFilePath, "/test/file"),
			},
		},
	}
	require.Equal(t, 1, logs.Len())
	require.Equal(t, expectedLogs, logs.AllUntimed())
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
	ctx := t.Context()
	testEntry := entry.New()
	now := time.Now()
	testEntry.Timestamp = now
	testEntry.AddAttribute(attrs.LogFilePath, "/test/file")
	transform := func(_ *entry.Entry) error {
		return errors.New("Failure")
	}

	err := transformer.ProcessWith(ctx, testEntry, transform)
	require.NoError(t, err)
	output.AssertNotCalled(t, "Process", mock.Anything, mock.Anything)

	// Test output logs
	expectedLogs := []observer.LoggedEntry{
		{
			Entry: zapcore.Entry{Level: zap.DebugLevel, Message: "Failed to process entry"},
			Context: []zapcore.Field{
				{Key: "error", Type: 26, Interface: errors.New("Failure")},
				zap.String("action", "drop_quiet"),
				zap.Time("entry.timestamp", now),
				zap.String(attrs.LogFilePath, "/test/file"),
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
	ctx := t.Context()
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
				zap.String("action", "send"),
				zap.Time("entry.timestamp", now),
				zap.String(attrs.LogFilePath, "/test/file"),
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
	ctx := t.Context()
	testEntry := entry.New()
	now := time.Now()
	testEntry.Timestamp = now
	testEntry.AddAttribute(attrs.LogFilePath, "/test/file")
	transform := func(_ *entry.Entry) error {
		return errors.New("Failure")
	}

	err := transformer.ProcessWith(ctx, testEntry, transform)
	require.NoError(t, err)
	output.AssertCalled(t, "Process", mock.Anything, mock.Anything)

	// Test output logs
	expectedLogs := []observer.LoggedEntry{
		{
			Entry: zapcore.Entry{Level: zap.DebugLevel, Message: "Failed to process entry"},
			Context: []zapcore.Field{
				{Key: "error", Type: 26, Interface: errors.New("Failure")},
				zap.String("action", "send_quiet"),
				zap.Time("entry.timestamp", now),
				zap.String(attrs.LogFilePath, "/test/file"),
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
	ctx := t.Context()
	testEntry := entry.New()
	transform := func(_ *entry.Entry) error {
		return nil
	}

	err := transformer.ProcessWith(ctx, testEntry, transform)
	require.NoError(t, err)
	output.AssertCalled(t, "Process", mock.Anything, mock.Anything)
}

// TestTransformerSplitsBatches documents that if a transformer operator uses the `TransformerOperator`'s `ProcessBatchWith` method,
// the batch is split into individual entries,
// as described in https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/39575.
func TestTransformerSplitsBatches(t *testing.T) {
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

	ctx := t.Context()
	testEntry := entry.New()
	testEntry2 := entry.New()
	testEntry3 := entry.New()
	testEntries := []*entry.Entry{testEntry, testEntry2, testEntry3}
	process := func(ctx context.Context, entries *entry.Entry) error {
		return transformer.ProcessWith(ctx, entries, func(*entry.Entry) error {
			return nil
		})
	}

	err := transformer.ProcessBatchWith(ctx, testEntries, process)
	require.NoError(t, err)
	// The following assertions prove that the batch was split into three separate calls to Process.
	output.AssertCalled(t, "Process", ctx, testEntry)
	output.AssertCalled(t, "Process", ctx, testEntry2)
	output.AssertCalled(t, "Process", ctx, testEntry3)
	output.AssertNumberOfCalls(t, "Process", 3)
}

// TestTransformerDoesNotSplitBatches documents that if a transformer operator uses the `TransformerOperator`'s `ProcessBatchWithTransform` method,
// the batch of entries is NOT split into individual entries, which is more performant and preferred way to process entries by operators.
func TestTransformerDoesNotSplitBatches(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

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

	ctx := t.Context()
	testEntry := entry.New()
	testEntry2 := entry.New()
	testEntry3 := entry.New()
	testEntries := []*entry.Entry{testEntry, testEntry2, testEntry3}
	transform := func(_ *entry.Entry) error {
		return nil
	}

	err := transformer.ProcessBatchWithTransform(ctx, testEntries, transform)
	require.NoError(t, err)
	// This proves that the batch was not split.
	output.AssertCalled(t, "ProcessBatch", ctx, testEntries)
	output.AssertNumberOfCalls(t, "ProcessBatch", 1)
}

// TestBatchProcessingAllEntriesFailQuietMode tests that when all entries fail in a batch with quiet mode,
// no errors are returned and entries are handled according to the quiet mode (drop or send).
func TestBatchProcessingAllEntriesFailQuietMode(t *testing.T) {
	testCases := []struct {
		name             string
		onError          string
		expectProcessed  bool
		expectError      bool
		expectedLogLevel zapcore.Level
	}{
		{
			name:             "DropOnErrorQuiet",
			onError:          DropOnErrorQuiet,
			expectProcessed:  false,
			expectError:      false,
			expectedLogLevel: zapcore.DebugLevel,
		},
		{
			name:             "SendOnErrorQuiet",
			onError:          SendOnErrorQuiet,
			expectProcessed:  true,
			expectError:      false,
			expectedLogLevel: zapcore.DebugLevel,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := &testutil.Operator{}
			output.On("ID").Return("test-output")
			output.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

			obs, logs := observer.New(zapcore.DebugLevel)
			set := componenttest.NewNopTelemetrySettings()
			set.Logger = zap.New(obs)

			transformer := TransformerOperator{
				OnError: tc.onError,
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

			ctx := t.Context()
			testEntry1 := entry.New()
			testEntry2 := entry.New()
			testEntry3 := entry.New()
			testEntries := []*entry.Entry{testEntry1, testEntry2, testEntry3}

			// Transform function that always fails
			transform := func(_ *entry.Entry) error {
				return errors.New("transform failure")
			}

			err := transformer.ProcessBatchWithTransform(ctx, testEntries, transform)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err, "expected no error in quiet mode")
			}

			// Verify that logs were created at the expected level
			require.Equal(t, 3, logs.Len(), "expected 3 log entries for 3 failed entries")
			for _, logEntry := range logs.All() {
				require.Equal(t, tc.expectedLogLevel, logEntry.Level, "expected log level to be %v", tc.expectedLogLevel)
			}

			// Verify ProcessBatch was called
			output.AssertCalled(t, "ProcessBatch", mock.Anything, mock.Anything)

			// Check if entries were sent or dropped
			calls := output.Calls
			if len(calls) > 0 {
				processedEntries := calls[0].Arguments.Get(1).([]*entry.Entry)
				if tc.expectProcessed {
					require.Len(t, processedEntries, 3, "expected all 3 entries to be sent in SendOnErrorQuiet mode")
				} else {
					require.Empty(t, processedEntries, "expected 0 entries to be sent in DropOnErrorQuiet mode")
				}
			}
		})
	}
}

// TestBatchProcessingMixedSuccessFailureQuietMode tests batch processing with a mix of successful and failed entries in quiet mode.
func TestBatchProcessingMixedSuccessFailureQuietMode(t *testing.T) {
	testCases := []struct {
		name                   string
		onError                string
		expectError            bool
		expectedProcessedCount int
		expectedLogLevel       zapcore.Level
	}{
		{
			name:                   "DropOnErrorQuiet",
			onError:                DropOnErrorQuiet,
			expectError:            false,
			expectedProcessedCount: 2, // Only successful entries
			expectedLogLevel:       zapcore.DebugLevel,
		},
		{
			name:                   "SendOnErrorQuiet",
			onError:                SendOnErrorQuiet,
			expectError:            false,
			expectedProcessedCount: 4, // All entries (2 successful + 2 failed)
			expectedLogLevel:       zapcore.DebugLevel,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := &testutil.Operator{}
			output.On("ID").Return("test-output")
			output.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

			obs, logs := observer.New(zapcore.DebugLevel)
			set := componenttest.NewNopTelemetrySettings()
			set.Logger = zap.New(obs)

			transformer := TransformerOperator{
				OnError: tc.onError,
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

			ctx := t.Context()
			testEntry1 := entry.New()
			testEntry1.Body = "success1"
			testEntry2 := entry.New()
			testEntry2.Body = "fail1"
			testEntry3 := entry.New()
			testEntry3.Body = "success2"
			testEntry4 := entry.New()
			testEntry4.Body = "fail2"
			testEntries := []*entry.Entry{testEntry1, testEntry2, testEntry3, testEntry4}

			// Transform function that fails for entries with "fail" in body
			transform := func(e *entry.Entry) error {
				if body, ok := e.Body.(string); ok && body[:4] == "fail" {
					return errors.New("transform failure")
				}
				return nil
			}

			err := transformer.ProcessBatchWithTransform(ctx, testEntries, transform)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err, "expected no error in quiet mode")
			}

			// Verify that logs were created for failed entries only
			require.Equal(t, 2, logs.Len(), "expected 2 log entries for 2 failed entries")
			for _, logEntry := range logs.All() {
				require.Equal(t, tc.expectedLogLevel, logEntry.Level, "expected log level to be %v", tc.expectedLogLevel)
			}

			// Verify ProcessBatch was called
			output.AssertCalled(t, "ProcessBatch", mock.Anything, mock.Anything)

			// Check the number of processed entries
			calls := output.Calls
			if len(calls) > 0 {
				processedEntries := calls[0].Arguments.Get(1).([]*entry.Entry)
				require.Len(t, processedEntries, tc.expectedProcessedCount,
					"expected %d entries to be processed", tc.expectedProcessedCount)
			}
		})
	}
}

// TestBatchProcessingAllEntriesFailNonQuietMode tests that errors are properly returned in non-quiet mode.
func TestBatchProcessingAllEntriesFailNonQuietMode(t *testing.T) {
	testCases := []struct {
		name             string
		onError          string
		expectProcessed  bool
		expectError      bool
		expectedLogLevel zapcore.Level
	}{
		{
			name:             "DropOnError",
			onError:          DropOnError,
			expectProcessed:  false,
			expectError:      true,
			expectedLogLevel: zapcore.ErrorLevel,
		},
		{
			name:             "SendOnError",
			onError:          SendOnError,
			expectProcessed:  true,
			expectError:      true,
			expectedLogLevel: zapcore.ErrorLevel,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := &testutil.Operator{}
			output.On("ID").Return("test-output")
			output.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

			obs, logs := observer.New(zapcore.WarnLevel)
			set := componenttest.NewNopTelemetrySettings()
			set.Logger = zap.New(obs)

			transformer := TransformerOperator{
				OnError: tc.onError,
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

			ctx := t.Context()
			testEntry1 := entry.New()
			testEntry2 := entry.New()
			testEntry3 := entry.New()
			testEntries := []*entry.Entry{testEntry1, testEntry2, testEntry3}

			// Transform function that always fails
			transform := func(_ *entry.Entry) error {
				return errors.New("transform failure")
			}

			err := transformer.ProcessBatchWithTransform(ctx, testEntries, transform)

			if tc.expectError {
				require.Error(t, err, "expected error in non-quiet mode")
				// Verify we get multiple errors joined together
				require.Contains(t, err.Error(), "transform failure")
			} else {
				require.NoError(t, err)
			}

			// Verify that logs were created at the expected level
			require.Equal(t, 3, logs.Len(), "expected 3 log entries for 3 failed entries")
			for _, logEntry := range logs.All() {
				require.Equal(t, tc.expectedLogLevel, logEntry.Level)
			}

			// Verify ProcessBatch was called
			output.AssertCalled(t, "ProcessBatch", mock.Anything, mock.Anything)

			// Check if entries were sent or dropped
			calls := output.Calls
			if len(calls) > 0 {
				processedEntries := calls[0].Arguments.Get(1).([]*entry.Entry)
				if tc.expectProcessed {
					require.Len(t, processedEntries, 3, "expected all 3 entries to be sent in SendOnError mode")
				} else {
					require.Empty(t, processedEntries, "expected 0 entries to be sent in DropOnError mode")
				}
			}
		})
	}
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
			err = transformer.ProcessWith(t.Context(), e, func(e *entry.Entry) error {
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
