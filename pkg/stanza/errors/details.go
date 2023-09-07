// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
