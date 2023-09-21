// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/tracetracker/shims.go

package tracetracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/tracetracker"

import (
	"github.com/signalfx/golib/v3/trace"
)

var (
	_ SpanList = (*spanListWrap)(nil)
	_ Span     = (*spanWrap)(nil)
)

// Span is a generic interface for accessing span metadata.
type Span interface {
	Environment() (string, bool)
	ServiceName() (string, bool)
	Tag(string) (string, bool)
	NumTags() int
}

// SpanList is a generic interface for accessing a list of spans.
type SpanList interface {
	Len() int
	At(i int) Span
}

type spanWrap struct {
	*trace.Span
}

func (s spanWrap) Environment() (string, bool) {
	env, ok := s.Tags["environment"]
	return env, ok
}

func (s spanWrap) ServiceName() (string, bool) {
	if s.LocalEndpoint == nil || s.LocalEndpoint.ServiceName == nil || *s.LocalEndpoint.ServiceName == "" {
		return "", false
	}
	return *s.LocalEndpoint.ServiceName, true
}

func (s spanWrap) Tag(tag string) (string, bool) {
	t, ok := s.Tags[tag]
	return t, ok
}

func (s spanWrap) NumTags() int {
	return len(s.Tags)
}

type spanListWrap struct {
	spans []*trace.Span
}

func (s spanListWrap) Len() int {
	return len(s.spans)
}

func (s spanListWrap) At(i int) Span {
	return spanWrap{Span: s.spans[i]}
}
