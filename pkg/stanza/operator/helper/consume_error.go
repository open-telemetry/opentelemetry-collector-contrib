// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

// ConsumeError is returned by a log emitter when the next consumer did not
// accept the entries, for example because an exporter failed. It lets an input
// operator tell a failed delivery apart from a parsing or transformation error,
// which is returned unwrapped. Use errors.As to detect it.
//
// The entries were already handed to the pipeline and may have been returned
// to the entry pool, so a caller must not write the same *entry.Entry again.
// To retry, read the data again from the source.
type ConsumeError struct {
	err error
}

func newConsumeError(err error) error {
	if err == nil {
		return nil
	}
	return &ConsumeError{err: err}
}

func (e *ConsumeError) Error() string {
	if e.err == nil {
		return "consume logs failed"
	}
	return "consume logs: " + e.err.Error()
}

// Unwrap returns the error reported by the next consumer.
func (e *ConsumeError) Unwrap() error {
	return e.err
}
