// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/errors"

// RecoverableError represents an error that can be ignored
// so that the receiver can continue to function.
type RecoverableError struct {
	Err error
}

func (e *RecoverableError) Error() string {
	return e.Err.Error()
}

// Unwrap implements the new error feature introduced in Go 1.13
func (e *RecoverableError) Unwrap() error {
	return e.Err
}

// IrrecoverableError represents an error that should
// stop the receiver.
type IrrecoverableError struct {
	Err error
}

func (e *IrrecoverableError) Error() string {
	return e.Err.Error()
}

// Unwrap implements the new error feature introduced in Go 1.13
func (e *IrrecoverableError) Unwrap() error {
	return e.Err
}
