// Copyright The OpenTelemetry Authors
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

package logzioexporter

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"

	"github.com/hashicorp/go-hclog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// hclog2ZapLogger implements Hashicorp's hclog.Logger interface using Uber's zap.Logger. It's a workaround for plugin
// system. go-plugin doesn't support other logger than hclog. This logger implements only methods used by the go-plugin.
type hclog2ZapLogger struct {
	Zap  *zap.Logger
	name string
}

func (l *hclog2ZapLogger) Log(level hclog.Level, msg string, args ...interface{}) {}

func (l *hclog2ZapLogger) ImpliedArgs() []interface{} {
	return nil
}

func (l *hclog2ZapLogger) Name() string {
	return l.name
}

func (l *hclog2ZapLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return nil
}

// Trace implementation.
func (l *hclog2ZapLogger) Trace(msg string, args ...interface{}) {}

// Debug implementation.
func (l *hclog2ZapLogger) Debug(msg string, args ...interface{}) {
	l.Zap.Debug(msg, argsToFields(args...)...)
}

// Info implementation.
func (l *hclog2ZapLogger) Info(msg string, args ...interface{}) {
	l.Zap.Info(msg, argsToFields(args...)...)
}

// Warn implementation.
func (l *hclog2ZapLogger) Warn(msg string, args ...interface{}) {
	l.Zap.Warn(msg, argsToFields(args...)...)
}

// Error implementation.
func (l *hclog2ZapLogger) Error(msg string, args ...interface{}) {
	l.Zap.Error(msg, argsToFields(args...)...)
}

// IsTrace implementation.
func (l *hclog2ZapLogger) IsTrace() bool { return false }

// IsDebug implementation.
func (l *hclog2ZapLogger) IsDebug() bool { return false }

// IsInfo implementation.
func (l *hclog2ZapLogger) IsInfo() bool { return false }

// IsWarn implementation.
func (l *hclog2ZapLogger) IsWarn() bool { return false }

// IsError implementation.
func (l *hclog2ZapLogger) IsError() bool { return false }

// With implementation.
func (l *hclog2ZapLogger) With(args ...interface{}) hclog.Logger {
	return &hclog2ZapLogger{Zap: l.Zap.With(argsToFields(args...)...)}
}

// Named implementation.
func (l *hclog2ZapLogger) Named(name string) hclog.Logger {
	return &hclog2ZapLogger{Zap: l.Zap.Named(name)}
}

// ResetNamed implementation.
func (l *hclog2ZapLogger) ResetNamed(name string) hclog.Logger {
	// no need to implement that as go-plugin doesn't use this method.
	return &hclog2ZapLogger{}
}

// SetLevel implementation.
func (l *hclog2ZapLogger) SetLevel(level hclog.Level) {
	// no need to implement that as go-plugin doesn't use this method.
}

// StandardLogger implementation.
func (l *hclog2ZapLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	// no need to implement that as go-plugin doesn't use this method.
	return log.New(ioutil.Discard, "", 0)
}

func argsToFields(args ...interface{}) []zapcore.Field {
	fields := []zapcore.Field{}
	for i := 0; i < len(args); i += 2 {
		fields = append(fields, zap.String(args[i].(string), fmt.Sprintf("%v", args[i+1])))
	}

	return fields
}
