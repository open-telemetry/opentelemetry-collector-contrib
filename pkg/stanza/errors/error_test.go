// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errors

import (
	"fmt"
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

		require.Equal(t, err2.Details, ErrorDetails{"foo": "bar"})
	})

	t.Run("AgentErrorWithExistingDetails", func(t *testing.T) {
		err := NewError("Test error", "", "foo1", "bar1")
		err2 := WithDetails(err, "foo2", "bar2")

		require.Equal(t, err2.Details, ErrorDetails{"foo1": "bar1", "foo2": "bar2"})
	})

	t.Run("StandardError", func(t *testing.T) {
		err := fmt.Errorf("Test error")
		err2 := WithDetails(err, "foo", "bar")

		require.Equal(t, err2.Details, ErrorDetails{"foo": "bar"})
	})

	t.Run("AgentMethod", func(t *testing.T) {
		err := NewError("Test error", "").WithDetails("foo", "bar")
		require.Equal(t, err.Details, ErrorDetails{"foo": "bar"})
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
		err := fmt.Errorf("Test error")
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

		expected := `{"level":"debug","ts":-6795364578.8713455,"logger":"testlogger","msg":"Got an error","error":{"description":"Test error"}}` + "\n"
		require.Equal(t, expected, out.String())
	})

	t.Run("SuggestionAndDetails", func(t *testing.T) {
		fields[0].Interface = NewError("Test error", "Fix it", "foo", "bar")
		out, err := enc.EncodeEntry(entry, fields)
		require.NoError(t, err)

		expected := `{"level":"debug","ts":-6795364578.8713455,"logger":"testlogger","msg":"Got an error","error":{"description":"Test error","suggestion":"Fix it","details":{"foo":"bar"}}}` + "\n"
		require.Equal(t, expected, out.String())
	})
}
