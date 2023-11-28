// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correlation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/correlation"

import (
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/tracetracker"
)

var (
	_ tracetracker.SpanList = (*spanListWrap)(nil)
	_ tracetracker.Span     = (*spanWrap)(nil)
)

type spanWrap struct {
	ptrace.ResourceSpans
}

func (s spanWrap) Environment() (string, bool) {
	attr := s.Resource().Attributes()

	// Try to find deployment.environment before falling back to environment (SignalFx value).
	env, ok := attr.Get(conventions.AttributeDeploymentEnvironment)
	if ok && env.Str() != "" {
		return env.Str(), true
	}

	env, ok = attr.Get("environment")
	if ok && env.Str() != "" {
		return env.Str(), true
	}

	return "", false
}

func (s spanWrap) ServiceName() (string, bool) {
	attr := s.Resource().Attributes()

	serviceName, ok := attr.Get(conventions.AttributeServiceName)
	if ok && serviceName.Str() != "" {
		return serviceName.Str(), true
	}

	return "", false
}

func (s spanWrap) Tag(tag string) (string, bool) {
	attr := s.Resource().Attributes()
	val, ok := attr.Get(tag)
	if ok {
		return val.Str(), true
	}
	return "", false
}

func (s spanWrap) NumTags() int {
	attr := s.Resource().Attributes()
	return attr.Len()
}

type spanListWrap struct {
	ptrace.ResourceSpansSlice
}

func (s spanListWrap) Len() int {
	return s.ResourceSpansSlice.Len()
}

func (s spanListWrap) At(i int) tracetracker.Span {
	return spanWrap{ResourceSpans: s.ResourceSpansSlice.At(i)}
}
