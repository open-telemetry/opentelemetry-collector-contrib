// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookup // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/lookup"

import (
	"errors"
	"fmt"
)

// ErrorMode defines how lookup errors are handled.
type ErrorMode string

const (
	// ErrorModeIgnore ignores lookup errors and continues processing.
	ErrorModeIgnore ErrorMode = "ignore"

	// ErrorModePropagate propagates lookup errors to the pipeline.
	ErrorModePropagate ErrorMode = "propagate"
)

// Validate checks if the error mode is valid.
func (e ErrorMode) Validate() error {
	switch e {
	case ErrorModeIgnore, ErrorModePropagate:
		return nil
	default:
		return fmt.Errorf("invalid error mode: %s", e)
	}
}

// ErrKeyNotFound is returned when a key is not present in the lookup source.
var ErrKeyNotFound = errors.New("key not found")
