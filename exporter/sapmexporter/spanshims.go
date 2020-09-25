// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sapmexporter

import (
	"github.com/signalfx/signalfx-agent/pkg/apm/tracetracker"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

var (
	_ tracetracker.SpanList = (*spanListWrap)(nil)
	_ tracetracker.Span     = (*spanWrap)(nil)
)

type spanWrap struct {
	pdata.ResourceSpans
}

func (s spanWrap) Environment() (string, bool) {
	res := s.Resource()
	if res.IsNil() {
		return "", false
	}
	attr := res.Attributes()

	// Try to find deployment.environment before falling back to environment (SignalFx value).
	env, ok := attr.Get(conventions.AttributeDeploymentEnvironment)
	if ok && env.StringVal() != "" {
		return env.StringVal(), true
	}

	env, ok = attr.Get("environment")
	if ok && env.StringVal() != "" {
		return env.StringVal(), true
	}

	return "", false
}

func (s spanWrap) ServiceName() (string, bool) {
	res := s.Resource()
	if res.IsNil() {
		return "", false
	}
	attr := res.Attributes()

	serviceName, ok := attr.Get(conventions.AttributeServiceName)
	if ok && serviceName.StringVal() != "" {
		return serviceName.StringVal(), true
	}

	return "", false
}

func (s spanWrap) Tag(tag string) (string, bool) {
	res := s.Resource()
	if res.IsNil() {
		return "", false
	}
	attr := res.Attributes()
	val, ok := attr.Get(tag)
	if ok {
		return val.StringVal(), true
	}
	return "", false
}

func (s spanWrap) NumTags() int {
	res := s.Resource()
	if res.IsNil() {
		return 0
	}
	attr := res.Attributes()
	return attr.Len()
}

type spanListWrap struct {
	pdata.ResourceSpansSlice
}

func (s spanListWrap) Len() int {
	return s.ResourceSpansSlice.Len()
}

func (s spanListWrap) At(i int) tracetracker.Span {
	return spanWrap{ResourceSpans: s.ResourceSpansSlice.At(i)}
}
