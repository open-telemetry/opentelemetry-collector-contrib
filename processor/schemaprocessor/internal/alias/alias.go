// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package alias is a subset of the interfaces defined by pdata and family
// package to allow for higher code reuse without using generics.
package alias // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Resource defines a minimal interface so that
// the change set can continue using the same pattern
type Resource interface {
	SchemaUrl() string

	SetSchemaUrl(url string)

	Resource() pcommon.Resource
}

// NamedSignal represents a subset of incoming pdata
// that can be updated using the schema processor
type NamedSignal interface {
	Name() string

	SetName(name string)
}

type Attributed interface {
	Attributes() pcommon.Map
}

var (
	_ Resource = (*plog.ResourceLogs)(nil)
	_ Resource = (*pmetric.ResourceMetrics)(nil)
	_ Resource = (*ptrace.ResourceSpans)(nil)

	_ NamedSignal = (*pmetric.Metric)(nil)
	_ NamedSignal = (*ptrace.Span)(nil)
	_ NamedSignal = (*ptrace.SpanEvent)(nil)
)

// AttributeKey is a type alias of string to help
// make clear what the strings being stored represent
type AttributeKey = string

// SignalName is a type alias of a string to help
// make clear what a type field is being used for.
type SignalName = string
