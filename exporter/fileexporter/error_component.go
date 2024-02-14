// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
)

// errorComponent is used to return error from a factory method. SharedComponent does
// not handle errors, so wrapping the error into a component is necessary.
type errorComponent struct {
	err error
}

// Start will return the cached error.
func (e *errorComponent) Start(context.Context, component.Host) error {
	return e.err
}

// Shutdown will return the cached error.
func (e *errorComponent) Shutdown(context.Context) error {
	return e.err
}
