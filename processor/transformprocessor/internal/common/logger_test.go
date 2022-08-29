// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func createLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zap.InfoLevel)
	return zap.New(core), logs
}

// Verify that WithFields properly stores Zap Field objects in the logger. We
// don't verify the field objects that are stored within the logger.
func Test_WithFields(t *testing.T) {
	tests := []struct {
		name   string
		fields []map[string]any
		msg    string
	}{
		{
			name:   "Empty case",
			fields: []map[string]any{},
			msg:    "",
		},
		{
			name:   "No fields",
			fields: []map[string]any{},
			msg:    "log message",
		},
		{
			name: "One WithFields call",
			fields: []map[string]any{
				{
					"string": "test",
					"int":    int64(1),
					"float":  1.0,
					"bool":   true,
					"[]byte": []byte{0, 1, 2, 3, 4, 5, 6, 7},
					"nil":    nil,
				},
			},
			msg: "log message",
		},
		{
			name: "Two WithFields calls",
			fields: []map[string]any{
				{
					"string": "test",
					"int":    int64(1),
					"float":  1.0,
					"bool":   true,
					"[]byte": []byte{0, 1, 2, 3, 4, 5, 6, 7},
					"nil":    nil,
				},
				{
					"string": "test1",
				},
			},
			msg: "log message",
		},
		{
			name: "Three WithFields calls",
			fields: []map[string]any{
				{
					"string": "test",
					"int":    int64(1),
					"float":  1.0,
					"bool":   true,
					"[]byte": []byte{0, 1, 2, 3, 4, 5, 6, 7},
					"nil":    nil,
				},
				{
					"string": "test1",
				},
				{
					"string": "test2",
					"int":    int64(2),
				},
			},
			msg: "log message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, logs := createLogger()
			tqll := NewTQLLogger(logger)

			for _, f := range tt.fields {
				tqll = tqll.WithFields(f).(TQLLogger)
				tqll.Info(tt.msg)
			}

			totalFields := 0

			for i, log := range logs.All() {
				totalFields += len(tt.fields[i])

				assert.Equal(t, totalFields, len(log.Context))
				assert.Equal(t, tt.msg, log.Message)
			}
		})
	}
}

func Test_WithFields_serialization(t *testing.T) {
	t.Run("Byte slices are serialized as hexadecimal", func(t *testing.T) {
		logger, logs := createLogger()
		tqll := NewTQLLogger(logger)
		byteSlice := []byte{0, 1, 2, 3, 4, 5, 6, 7}

		tqll.WithFields(map[string]any{"byteSlice": byteSlice}).Info("log message")

		log := logs.All()[0]

		assert.Equal(t, log.Context[0].String, fmt.Sprintf("%x", byteSlice))
	})
}

func Test_logger_logging(t *testing.T) {
	tests := []struct {
		name      string
		loglevels []string
		log       func(tqll TQLLogger, msg string)
	}{
		{
			name:      "Info",
			loglevels: []string{"info"},
			log: func(tqll TQLLogger, msg string) {
				tqll.Info(msg)
			},
		},
		{
			name:      "Error",
			loglevels: []string{"error"},
			log: func(tqll TQLLogger, msg string) {
				tqll.Error(msg)
			},
		},
		{
			name:      "Multiple Info",
			loglevels: []string{"info", "info", "info"},
			log: func(tqll TQLLogger, msg string) {
				tqll.Info(msg)
				tqll.Info(msg)
				tqll.Info(msg)
			},
		},
		{
			name:      "Multiple Error",
			loglevels: []string{"error", "error", "error"},
			log: func(tqll TQLLogger, msg string) {
				tqll.Error(msg)
				tqll.Error(msg)
				tqll.Error(msg)
			},
		},
		{
			name:      "Mixed loglevels",
			loglevels: []string{"info", "error", "info"},
			log: func(tqll TQLLogger, msg string) {
				tqll.Info(msg)
				tqll.Error(msg)
				tqll.Info(msg)
			},
		},
	}

	for _, tt := range tests {
		logger, logs := createLogger()
		tqll := NewTQLLogger(logger)

		tt.log(tqll, "testing")

		for i, log := range logs.All() {
			assert.Equal(t, tt.loglevels[i], log.Level.String())
		}
	}
}
