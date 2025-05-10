// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"fmt"
)

func Error(id Ident, err error) error {
	return StreamErr{Ident: id, Err: err}
}

type StreamErr struct {
	Ident Ident
	Err   error
}

func (e StreamErr) Error() string {
	return fmt.Sprintf("%s: %s", e.Ident, e.Err)
}

func (e StreamErr) Unwrap() error {
	return e.Err
}
