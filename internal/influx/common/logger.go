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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/common"

// Logger must be implemented by the user of this package.
// Emitted logs indicate non-fatal conversion errors.
type Logger interface {
	Debug(msg string, kv ...interface{})
}

// NoopLogger is a no-op implementation of Logger.
type NoopLogger struct{}

func (NoopLogger) Debug(_ string, _ ...interface{}) {}

// ErrorLogger intercepts log entries emitted by this package,
// adding key "error" before any error type value.
//
// ErrorLogger panicks if the resulting kv slice length is odd.
type ErrorLogger struct {
	Logger
}

func (e *ErrorLogger) Debug(msg string, kv ...interface{}) {
	for i := range kv {
		if _, isError := kv[i].(error); isError {
			kv = append(kv, nil)
			copy(kv[i+1:], kv[i:])
			kv[i] = "error"
		}
	}
	if len(kv)%2 != 0 {
		panic("log entry kv count is odd")
	}
	e.Logger.Debug(msg, kv...)
}
