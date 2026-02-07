// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stanzaerrors

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestWithDetails(t *testing.T) {
	t.Run("AgentErrorWithNoExistingDetails", func(t *testing.T) {
		err := NewError("Test error", "")
		err2 := WithDetails(err, "foo", "bar")

		require.Equal(t, ErrorDetails{"foo": "bar"}, err2.Details)
	})

	t.Run("AgentErrorWithExistingDetails", func(t *testing.T) {
		err := NewError("Test error", "", "foo1", "bar1")
		err2 := WithDetails(err, "foo2", "bar2")

		require.Equal(t, ErrorDetails{"foo1": "bar1", "foo2": "bar2"}, err2.Details)
	})

	t.Run("StandardError", func(t *testing.T) {
		err := errors.New("Test error")
		err2 := WithDetails(err, "foo", "bar")

		require.Equal(t, ErrorDetails{"foo": "bar"}, err2.Details)
	})

	t.Run("AgentMethod", func(t *testing.T) {
		err := NewError("Test error", "").WithDetails("foo", "bar")
		require.Equal(t, ErrorDetails{"foo": "bar"}, err.Details)
	})
}

func TestErrorMessage(t *testing.T) {
	t.Run("WithDetails", func(t *testing.T) {
		err := NewError("Test error", "", "foo", "bar")

		require.Equal(t, `Test error: {"foo":"bar"}`, err.Error())
	})

	t.Run("WithoutDetails", func(t *testing.T) {
		err := NewError("Test error", "")

		require.Equal(t, `Test error`, err.Error())
	})
}

func TestWrap(t *testing.T) {
	t.Run("AgentError", func(t *testing.T) {
		err := NewError("Test error", "")
		err2 := Wrap(err, "Test context")
		require.Equal(t, "Test context: Test error", err2.Error())
	})

	t.Run("StandardError", func(t *testing.T) {
		err := errors.New("Test error")
		err2 := Wrap(err, "Test context")
		require.Equal(t, "Test context: Test error", err2.Error())
	})
}

func TestMarshalLogObject(t *testing.T) {
	cfg := zap.NewProductionConfig()
	enc := zapcore.NewJSONEncoder(cfg.EncoderConfig)
	now, _ := time.Parse(time.RFC3339, time.RFC3339)
	entry := zapcore.Entry{
		Level:      zapcore.DebugLevel,
		Time:       now,
		LoggerName: "testlogger",
		Message:    "Got an error",
	}
	fields := []zapcore.Field{{
		Key:  "error",
		Type: zapcore.ObjectMarshalerType,
	}}

	t.Run("NoSuggestionOrDetails", func(t *testing.T) {
		fields[0].Interface = NewError("Test error", "")
		out, err := enc.EncodeEntry(entry, fields)
		require.NoError(t, err)

		expected := `{"level":"debug","logger":"testlogger","msg":"Got an error","error":{"description":"Test error"}}` + "\n"
		require.Equal(t, expected, out.String())
	})

	t.Run("SuggestionAndDetails", func(t *testing.T) {
		fields[0].Interface = NewError("Test error", "Fix it", "foo", "bar")
		out, err := enc.EncodeEntry(entry, fields)
		require.NoError(t, err)

		expected := `{"level":"debug","logger":"testlogger","msg":"Got an error","error":{"description":"Test error","suggestion":"Fix it","details":{"foo":"bar"}}}` + "\n"
		require.Equal(t, expected, out.String())
	})
}
