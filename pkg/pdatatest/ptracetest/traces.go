// Copyright The OpenTelemetry Authors
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

package ptracetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
)

// CompareTraces compares each part of two given Traces and returns
// an error if they don't match. The error describes what didn't match.
func CompareTraces(expected, actual ptrace.Traces, options ...CompareTracesOption) error {
	exp, act := ptrace.NewTraces(), ptrace.NewTraces()
	expected.CopyTo(exp)
	actual.CopyTo(act)

	for _, option := range options {
		option.applyOnTraces(exp, act)
	}

	expectedSpans, actualSpans := exp.ResourceSpans(), act.ResourceSpans()
	if expectedSpans.Len() != actualSpans.Len() {
		return fmt.Errorf("number of resources doesn't match expected: %d, actual: %d",
			expectedSpans.Len(),
			actualSpans.Len())
	}

	numResources := expectedSpans.Len()

	// Keep track of matching resources so that each can only be matched once
	matchingResources := make(map[ptrace.ResourceSpans]ptrace.ResourceSpans, numResources)

	var errs error
	var outOfOrderErrs error
	for e := 0; e < numResources; e++ {
		er := expectedSpans.At(e)
		var foundMatch bool
		for a := 0; a < numResources; a++ {
			ar := actualSpans.At(a)
			if _, ok := matchingResources[ar]; ok {
				continue
			}
			if reflect.DeepEqual(er.Resource().Attributes().AsRaw(), ar.Resource().Attributes().AsRaw()) {
				foundMatch = true
				matchingResources[ar] = er
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`resources are out of order: resource "%v" expected at index %d, found at index %d`,
							er.Resource().Attributes().AsRaw(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected resource: %v", er.Resource().Attributes().AsRaw()))
		}
	}

	for i := 0; i < numResources; i++ {
		if _, ok := matchingResources[actualSpans.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected resource: %v", actualSpans.At(i).Resource().Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for ar, er := range matchingResources {
		errPrefix := fmt.Sprintf(`resource "%v"`, er.Resource().Attributes().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareResourceSpans(er, ar)))
	}

	return errs
}

// CompareResourceSpans compares each part of two given ResourceSpans and returns
// an error if they don't match. The error describes what didn't match.
func CompareResourceSpans(expected, actual ptrace.ResourceSpans) error {
	var errs error

	if !reflect.DeepEqual(expected.Resource().Attributes().AsRaw(), actual.Resource().Attributes().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("attributes don't match expected: %v, actual: %v",
			expected.Resource().Attributes().AsRaw(), actual.Resource().Attributes().AsRaw()))
	}

	if expected.ScopeSpans().Len() != actual.ScopeSpans().Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of scopes doesn't match expected: %d, actual: %d",
			expected.ScopeSpans().Len(), actual.ScopeSpans().Len()))
		return errs
	}

	numScopeSpans := expected.ScopeSpans().Len()

	// Keep track of matching scope logs so that each record can only be matched once
	matchingScopeSpans := make(map[ptrace.ScopeSpans]ptrace.ScopeSpans, numScopeSpans)

	var outOfOrderErrs error
	for e := 0; e < numScopeSpans; e++ {
		es := expected.ScopeSpans().At(e)
		var foundMatch bool
		for a := 0; a < numScopeSpans; a++ {
			as := actual.ScopeSpans().At(a)
			if _, ok := matchingScopeSpans[as]; ok {
				continue
			}
			if es.Scope().Name() == as.Scope().Name() {
				foundMatch = true
				matchingScopeSpans[as] = es
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf("scopes are out of order: scope %s expected at index %d, found at index %d",
							es.Scope().Name(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected scope: %s", es.Scope().Name()))
		}
	}

	for i := 0; i < numScopeSpans; i++ {
		if _, ok := matchingScopeSpans[actual.ScopeSpans().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected scope: %s", actual.ScopeSpans().At(i).Scope().Name()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for i := 0; i < expected.ScopeSpans().Len(); i++ {
		errPrefix := fmt.Sprintf(`scope "%s"`, expected.ScopeSpans().At(i).Scope().Name())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix,
			CompareScopeSpans(expected.ScopeSpans().At(i), actual.ScopeSpans().At(i))))
	}
	return errs
}

// CompareScopeSpans compares each part of two given SpanSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareScopeSpans(expected, actual ptrace.ScopeSpans) error {
	var errs error

	if expected.Scope().Name() != actual.Scope().Name() {
		errs = multierr.Append(errs, fmt.Errorf("name doesn't match expected: %s, actual: %s",
			expected.Scope().Name(), actual.Scope().Name()))
	}
	if expected.Scope().Version() != actual.Scope().Version() {
		errs = multierr.Append(errs, fmt.Errorf("version doesn't match expected: %s, actual: %s",
			expected.Scope().Version(), actual.Scope().Version()))
	}

	if expected.Spans().Len() != actual.Spans().Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of spans doesn't match expected: %d, actual: %d",
			expected.Spans().Len(), actual.Spans().Len()))
		return errs
	}

	numSpans := expected.Spans().Len()

	// Keep track of matching spans so that each span can only be matched once
	matchingSpans := make(map[ptrace.Span]ptrace.Span, numSpans)

	var outOfOrderErrs error
	for e := 0; e < numSpans; e++ {
		es := expected.Spans().At(e)
		var foundMatch bool
		for a := 0; a < numSpans; a++ {
			as := actual.Spans().At(a)
			if _, ok := matchingSpans[as]; ok {
				continue
			}
			if es.Name() == as.Name() {
				foundMatch = true
				matchingSpans[as] = es
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`spans are out of order: span "%s" expected at index %d, found at index %d`,
							es.Name(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected span: %s", es.Name()))
		}
	}

	for i := 0; i < numSpans; i++ {
		if _, ok := matchingSpans[actual.Spans().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected span: %s", actual.Spans().At(i).Name()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for as, es := range matchingSpans {
		errs = multierr.Append(errs, internal.AddErrPrefix(fmt.Sprintf(`span "%s"`, es.Name()), CompareSpan(as, es)))
	}

	return errs
}

// CompareSpan compares each part of two given Span and returns
// an error if they don't match. The error describes what didn't match.
func CompareSpan(expected, actual ptrace.Span) error {
	var errs error

	if !reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("attributes don't match expected: %v, actual: %v",
			expected.Attributes().AsRaw(), actual.Attributes().AsRaw()))
	}

	if expected.TraceID() != actual.TraceID() {
		errs = multierr.Append(errs, fmt.Errorf("trace ID doesn't match expected: %d, actual: %d",
			expected.TraceID(), actual.TraceID()))
	}

	if expected.SpanID() != actual.SpanID() {
		errs = multierr.Append(errs, fmt.Errorf("span ID doesn't match expected: %d, actual: %d",
			expected.SpanID(), actual.SpanID()))
	}

	if expected.TraceState().AsRaw() != actual.TraceState().AsRaw() {
		errs = multierr.Append(errs, fmt.Errorf("trace state doesn't match expected: %s, actual: %s",
			expected.TraceState().AsRaw(), actual.TraceState().AsRaw()))
	}

	if expected.ParentSpanID() != actual.ParentSpanID() {
		errs = multierr.Append(errs, fmt.Errorf("parent span ID doesn't match expected: %d, actual: %d",
			expected.ParentSpanID(), actual.ParentSpanID()))
	}

	if expected.Name() != actual.Name() {
		errs = multierr.Append(errs, fmt.Errorf("name doesn't match expected: %s, actual: %s", expected.Name(),
			actual.Name()))
	}

	if expected.Kind() != actual.Kind() {
		errs = multierr.Append(errs, fmt.Errorf("kind doesn't match expected: %d, actual: %d", expected.Kind(),
			actual.Kind()))
	}

	if expected.StartTimestamp() != actual.StartTimestamp() {
		errs = multierr.Append(errs, fmt.Errorf("start timestamp doesn't match expected: %d, actual: %d",
			expected.StartTimestamp(), actual.StartTimestamp()))
	}

	if expected.EndTimestamp() != actual.EndTimestamp() {
		errs = multierr.Append(errs, fmt.Errorf("end timestamp doesn't match expected: %d, actual: %d",
			expected.EndTimestamp(), actual.EndTimestamp()))
	}

	if expected.DroppedAttributesCount() != actual.DroppedAttributesCount() {
		errs = multierr.Append(errs, fmt.Errorf("dropped attributes count doesn't match expected: %d, actual: %d",
			expected.DroppedAttributesCount(), actual.DroppedAttributesCount()))
	}

	if !reflect.DeepEqual(expected.Events(), actual.Events()) {
		errs = multierr.Append(errs, fmt.Errorf("events doesn't match expected: %v, actual: %v",
			expected.Events(), actual.Events()))
	}

	if expected.DroppedEventsCount() != actual.DroppedEventsCount() {
		errs = multierr.Append(errs, fmt.Errorf("dropped events count doesn't match expected: %d, actual: %d",
			expected.DroppedEventsCount(), actual.DroppedEventsCount()))
	}

	if !reflect.DeepEqual(expected.Links(), actual.Links()) {
		errs = multierr.Append(errs, fmt.Errorf("links doesn't match expected: %v, actual: %v",
			expected.Links(), actual.Links()))
	}

	if expected.DroppedLinksCount() != actual.DroppedLinksCount() {
		errs = multierr.Append(errs, fmt.Errorf("dropped links count doesn't match expected: %d, actual: %d",
			expected.DroppedLinksCount(), actual.DroppedLinksCount()))
	}

	if !reflect.DeepEqual(expected.Status(), actual.Status()) {
		errs = multierr.Append(errs, fmt.Errorf("status doesn't match expected: %v, actual: %v",
			expected.Status(), actual.Status()))
	}

	return errs
}
