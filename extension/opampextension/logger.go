// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package opampextension

import "go.uber.org/zap"

type Logger struct {
	Logger *zap.SugaredLogger
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	l.Logger.Debugf(format, v...)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.Logger.Errorf(format, v...)
}
