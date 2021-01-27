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
	"go.uber.org/zap"
)

// Logger is a wrapped logger used by the stanza agent.
type Logger struct {
	*zap.SugaredLogger
	*Emitter
}

// New will create a new logger.
func New(sugared *zap.SugaredLogger) *Logger {
	baseLogger := sugared.Desugar()
	emitter := newEmitter()
	core := newCore(baseLogger.Core(), emitter)
	wrappedLogger := zap.New(core).Sugar()

	return &Logger{
		wrappedLogger,
		emitter,
	}
}
