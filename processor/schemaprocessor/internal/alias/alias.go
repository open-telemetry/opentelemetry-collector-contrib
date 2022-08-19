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

// Package Alias is a subset of the interfaces defined by pdata and family
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

// Signal represents a subset of incoming pdata
// that can be updated using the schema processor
type Signal interface {
	Name() string

	SetName(name string)
}

var (
	_ Resource = (*plog.ResourceLogs)(nil)
	_ Resource = (*pmetric.ResourceMetrics)(nil)
	_ Resource = (*ptrace.ResourceSpans)(nil)

	_ Signal = (*pmetric.Metric)(nil)
	_ Signal = (*ptrace.Span)(nil)
	_ Signal = (*ptrace.SpanEvent)(nil)
)
