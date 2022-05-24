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

package errors // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"

import "go.uber.org/zap/zapcore"

// ErrorDetails is a map of details for an agent error.
type ErrorDetails map[string]string

// MarshalLogObject will define the representation of details when logging.
func (d ErrorDetails) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	for key, value := range d {
		encoder.AddString(key, value)
	}
	return nil
}

// createDetails will create details for an error from key/value pairs.
func createDetails(keyValues []string) ErrorDetails {
	details := make(ErrorDetails)
	if len(keyValues) > 0 {
		for i := 0; i+1 < len(keyValues); i += 2 {
			details[keyValues[i]] = keyValues[i+1]
		}
	}
	return details
}
