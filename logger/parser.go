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

package logger

import (
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
)

// parseEntry will create a stanza entry from a zapcore entry.
func parseEntry(zapEntry zapcore.Entry, fields []zapcore.Field) entry.Entry {
	return entry.Entry{
		Timestamp: zapEntry.Time,
		Record:    parseRecord(zapEntry, fields),
		Severity:  parseSeverity(zapEntry),
	}
}

// parseRecord will parse a record from a zapcore entry.
func parseRecord(zapEntry zapcore.Entry, fields []zapcore.Field) map[string]interface{} {
	encoder := zapcore.NewMapObjectEncoder()
	encoder.AddString("message", zapEntry.Message)

	for _, field := range fields {
		field.AddTo(encoder)
	}

	return encoder.Fields
}

// parseSeverity will parse a stanza severity from a zapcore entry.
func parseSeverity(zapEntry zapcore.Entry) entry.Severity {
	switch zapEntry.Level {
	case zapcore.DebugLevel:
		return entry.Debug
	case zapcore.InfoLevel:
		return entry.Info
	case zapcore.WarnLevel:
		return entry.Warning
	case zapcore.ErrorLevel:
		return entry.Error
	case zapcore.PanicLevel:
		return entry.Critical
	case zapcore.FatalLevel:
		return entry.Catastrophe
	default:
		return entry.Default
	}
}
