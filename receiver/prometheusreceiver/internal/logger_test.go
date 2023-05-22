// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLog(t *testing.T) {
	tcs := []struct {
		name        string
		input       []interface{}
		wantLevel   zapcore.Level
		wantMessage string
	}{
		{
			name: "Starting provider",
			input: []interface{}{
				"level",
				level.DebugValue(),
				"msg",
				"Starting provider",
				"provider",
				"string/0",
				"subs",
				"[target1]",
			},
			wantLevel:   zapcore.DebugLevel,
			wantMessage: "Starting provider",
		},
		{
			name: "Scrape failed",
			input: []interface{}{
				"level",
				level.ErrorValue(),
				"scrape_pool",
				"target1",
				"msg",
				"Scrape failed",
				"err",
				"server returned HTTP status 500 Internal Server Error",
			},
			wantLevel:   zapcore.ErrorLevel,
			wantMessage: "Scrape failed",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			conf := zap.NewProductionConfig()
			conf.Level.SetLevel(zapcore.DebugLevel)

			// capture zap log entry
			var entry zapcore.Entry
			h := func(e zapcore.Entry) error {
				entry = e
				return nil
			}

			logger, err := conf.Build(zap.Hooks(h))
			require.NoError(t, err)

			adapter := NewZapToGokitLogAdapter(logger)
			err = adapter.Log(tc.input...)
			require.NoError(t, err)

			assert.Equal(t, tc.wantLevel, entry.Level)
			assert.Equal(t, tc.wantMessage, entry.Message)
		})
	}
}

func TestExtractLogData(t *testing.T) {
	tcs := []struct {
		name        string
		input       []interface{}
		wantLevel   level.Value
		wantMessage string
		wantOutput  []interface{}
	}{
		{
			name:        "nil fields",
			input:       nil,
			wantLevel:   level.InfoValue(), // Default
			wantMessage: "",
			wantOutput:  nil,
		},
		{
			name:        "empty fields",
			input:       []interface{}{},
			wantLevel:   level.InfoValue(), // Default
			wantMessage: "",
			wantOutput:  nil,
		},
		{
			name: "info level",
			input: []interface{}{
				"level",
				level.InfoValue(),
			},
			wantLevel:   level.InfoValue(),
			wantMessage: "",
			wantOutput:  nil,
		},
		{
			name: "warn level",
			input: []interface{}{
				"level",
				level.WarnValue(),
			},
			wantLevel:   level.WarnValue(),
			wantMessage: "",
			wantOutput:  nil,
		},
		{
			name: "error level",
			input: []interface{}{
				"level",
				level.ErrorValue(),
			},
			wantLevel:   level.ErrorValue(),
			wantMessage: "",
			wantOutput:  nil,
		},
		{
			name: "debug level + extra fields",
			input: []interface{}{
				"timestamp",
				1596604719,
				"level",
				level.DebugValue(),
				"msg",
				"http client error",
			},
			wantLevel:   level.DebugValue(),
			wantMessage: "http client error",
			wantOutput: []interface{}{
				"timestamp", 1596604719,
			},
		},
		{
			name: "missing level field",
			input: []interface{}{
				"timestamp",
				1596604719,
				"msg",
				"http client error",
			},
			wantLevel:   level.InfoValue(), // Default
			wantMessage: "http client error",
			wantOutput: []interface{}{
				"timestamp", 1596604719,
			},
		},
		{
			name: "invalid level type",
			input: []interface{}{
				"level",
				"warn", // String is not recognized
			},
			wantLevel: level.InfoValue(), // Default
			wantOutput: []interface{}{
				"level", "warn", // Field is preserved
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ld := extractLogData(tc.input)
			assert.Equal(t, tc.wantLevel, ld.level)
			assert.Equal(t, tc.wantMessage, ld.msg)
			assert.Equal(t, tc.wantOutput, ld.otherFields)
		})
	}
}

func TestE2E(t *testing.T) {
	logger, observed := observer.New(zap.DebugLevel)
	gLogger := NewZapToGokitLogAdapter(zap.New(logger))

	const targetStr = "https://host.docker.internal:5000/prometheus"

	tcs := []struct {
		name        string
		log         func() error
		wantLevel   zapcore.Level
		wantMessage string
		wantOutput  []zapcore.Field
	}{
		{
			name: "debug level",
			log: func() error {
				return level.Debug(gLogger).Log()
			},
			wantLevel:   zapcore.DebugLevel,
			wantMessage: "",
			wantOutput:  []zapcore.Field{},
		},
		{
			name: "info level",
			log: func() error {
				return level.Info(gLogger).Log()
			},
			wantLevel:   zapcore.InfoLevel,
			wantMessage: "",
			wantOutput:  []zapcore.Field{},
		},
		{
			name: "warn level",
			log: func() error {
				return level.Warn(gLogger).Log()
			},
			wantLevel:   zapcore.WarnLevel,
			wantMessage: "",
			wantOutput:  []zapcore.Field{},
		},
		{
			name: "error level",
			log: func() error {
				return level.Error(gLogger).Log()
			},
			wantLevel:   zapcore.ErrorLevel,
			wantMessage: "",
			wantOutput:  []zapcore.Field{},
		},
		{
			name: "logger with and msg",
			log: func() error {
				ngLogger := log.With(gLogger, "scrape_pool", "scrape_pool")
				ngLogger = log.With(ngLogger, "target", targetStr)
				return level.Debug(ngLogger).Log("msg", "http client error", "err", fmt.Errorf("%s %q: dial tcp 192.168.65.2:5000: connect: connection refused", http.MethodGet, targetStr))
			},
			wantLevel:   zapcore.DebugLevel,
			wantMessage: "http client error",
			wantOutput: []zapcore.Field{
				zap.String("scrape_pool", "scrape_pool"),
				zap.String("target", "https://host.docker.internal:5000/prometheus"),
				zap.Error(fmt.Errorf("%s %q: dial tcp 192.168.65.2:5000: connect: connection refused", http.MethodGet, targetStr)),
			},
		},
		{
			name: "missing level",
			log: func() error {
				ngLogger := log.With(gLogger, "target", "foo")
				return ngLogger.Log("msg", "http client error")
			},
			wantLevel:   zapcore.InfoLevel, // Default
			wantMessage: "http client error",
			wantOutput: []zapcore.Field{
				zap.String("target", "foo"),
			},
		},
		{
			name: "invalid level type",
			log: func() error {
				ngLogger := log.With(gLogger, "target", "foo")
				return ngLogger.Log("msg", "http client error", "level", "warn")
			},
			wantLevel:   zapcore.InfoLevel, // Default
			wantMessage: "http client error",
			wantOutput: []zapcore.Field{
				zap.String("target", "foo"),
				zap.String("level", "warn"), // Field is preserved
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, tc.log())
			entries := observed.TakeAll()
			require.Len(t, entries, 1)
			assert.Equal(t, tc.wantLevel, entries[0].Level)
			assert.Equal(t, tc.wantMessage, entries[0].Message)
			assert.Equal(t, tc.wantOutput, entries[0].Context)
		})
	}
}
