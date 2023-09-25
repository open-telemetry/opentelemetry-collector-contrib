// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/tracetracker/shims.go

package tracetracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/tracetracker"

var (
	_ SpanList = (*fakeSpanList)(nil)
	_ Span     = (*fakeSpan)(nil)
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

type fakeSpan struct {
	serviceName string
	tags        map[string]string
}

func (s fakeSpan) Environment() (string, bool) {
	env, ok := s.tags["environment"]
	return env, ok
}

func (s fakeSpan) ServiceName() (string, bool) {
	return s.serviceName, s.serviceName != ""
}

func (s fakeSpan) Tag(tag string) (string, bool) {
	t, ok := s.tags[tag]
	return t, ok
}

func (s fakeSpan) NumTags() int {
	return len(s.tags)
}

type fakeSpanList []fakeSpan

func (s fakeSpanList) Len() int {
	return len(s)
}

func (s fakeSpanList) At(i int) Span {
	return s[i]
}
