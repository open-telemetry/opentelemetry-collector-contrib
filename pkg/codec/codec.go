// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package codec // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/codec"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Metric codec marshals and unmarshals metric pdata.
type Metric interface {
	pmetric.Marshaler
	pmetric.Unmarshaler
}

// Log codec marshals and unmarshals log pdata.
type Log interface {
	plog.Marshaler
	plog.Unmarshaler
}

// Trace codec marshals and unmarshals trace pdata.
type Trace interface {
	ptrace.Marshaler
	ptrace.Unmarshaler
}
