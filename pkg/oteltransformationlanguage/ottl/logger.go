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

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/ottl"

// Logger allows printing logs inside OTTL functions using a
// logging framework provided by the component using the OTTL.
type Logger interface {
	WithFields(field map[string]any) Logger
	Info(msg string)
	Error(msg string)
}

type NoOpLogger struct{}

func (nol NoOpLogger) WithFields(field map[string]any) Logger {
	return nol
}

func (nol NoOpLogger) Info(msg string) {
	// no-op
}

func (nol NoOpLogger) Error(msg string) {
	// no-op
}
