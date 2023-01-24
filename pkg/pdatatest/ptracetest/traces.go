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
		return fmt.Errorf("amount of ResourceSpans between Traces are not equal expected: %d, actual: %d",
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
						fmt.Errorf("ResourceTraces with attributes %v expected at index %d, "+
							"found at index %d", er.Resource().Attributes().AsRaw(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected resource with attributes: %v", er.Resource().Attributes().AsRaw()))
		}
	}

	for i := 0; i < numResources; i++ {
		if _, ok := matchingResources[actualSpans.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("extra resource with attributes: %v", actualSpans.At(i).Resource().Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for ar, er := range matchingResources {
		if err := CompareResourceSpans(er, ar); err != nil {
			return multierr.Combine(fmt.Errorf("ResourceSpans with attributes %v does not match expected",
				er.Resource().Attributes().AsRaw()), err)
		}
	}

	return nil
}

// CompareResourceSpans compares each part of two given ResourceSpans and returns
// an error if they don't match. The error describes what didn't match.
func CompareResourceSpans(expected, actual ptrace.ResourceSpans) error {
	if !reflect.DeepEqual(expected.Resource().Attributes().AsRaw(), actual.Resource().Attributes().AsRaw()) {
		return fmt.Errorf("resource attributes do not match expected: %v, actual: %v",
			expected.Resource().Attributes().AsRaw(), actual.Resource().Attributes().AsRaw())
	}

	if expected.ScopeSpans().Len() != actual.ScopeSpans().Len() {
		return fmt.Errorf("number of scope spans does not match expected: %d, actual: %d",
			expected.ScopeSpans().Len(),
			actual.ScopeSpans().Len())
	}

	numScopeSpans := expected.ScopeSpans().Len()

	// Keep track of matching scope logs so that each record can only be matched once
	matchingScopeSpans := make(map[ptrace.ScopeSpans]ptrace.ScopeSpans, numScopeSpans)

	var errs error
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
						fmt.Errorf("ScopeSpans with scope name %s expected at index %d, found at index %d",
							es.Scope().Name(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("ScopeSpans missing with scope name: %s", es.Scope().Name()))
		}
	}

	for i := 0; i < numScopeSpans; i++ {
		if _, ok := matchingScopeSpans[actual.ScopeSpans().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected ScopeSpans with scope name: %s",
				actual.ScopeSpans().At(i).Scope().Name()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for i := 0; i < expected.ScopeSpans().Len(); i++ {
		if err := CompareScopeSpans(expected.ScopeSpans().At(i), actual.ScopeSpans().At(i)); err != nil {
			return multierr.Combine(fmt.Errorf(`ScopeSpans with name "%s" does not match expected`,
				expected.ScopeSpans().At(i).Scope().Name()), err)
		}
	}
	return nil
}

// CompareScopeSpans compares each part of two given SpanSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareScopeSpans(expected, actual ptrace.ScopeSpans) error {
	if expected.Scope().Name() != actual.Scope().Name() {
		return fmt.Errorf("scope Name does not match expected: %s, actual: %s",
			expected.Scope().Name(), actual.Scope().Name())
	}
	if expected.Scope().Version() != actual.Scope().Version() {
		return fmt.Errorf("scope Version does not match expected: %s, actual: %s",
			expected.Scope().Version(), actual.Scope().Version())
	}

	if expected.Spans().Len() != actual.Spans().Len() {
		return fmt.Errorf("number of spans does not match expected: %d, actual: %d",
			expected.Spans().Len(), actual.Spans().Len())
	}

	numSpans := expected.Spans().Len()

	// Keep track of matching spans so that each span can only be matched once
	matchingSpans := make(map[ptrace.Span]ptrace.Span, numSpans)

	var errs error
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
						fmt.Errorf("span %s expected at index %d, found at index %d", es.Name(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected span %s", es.Name()))
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
		if err := CompareSpan(as, es); err != nil {
			return multierr.Combine(fmt.Errorf("span %s does not match expected", as.Name()), err)
		}
	}
	return nil
}

// CompareSpan compares each part of two given Span and returns
// an error if they don't match. The error describes what didn't match.
func CompareSpan(expected, actual ptrace.Span) error {
	if !reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) {
		return fmt.Errorf("span attributes do not match expected: %v, actual: %v",
			expected.Attributes().AsRaw(), actual.Attributes().AsRaw())
	}
	if expected.TraceID() != actual.TraceID() {
		return fmt.Errorf("span TraceID doesn't match expected: %d, actual: %d",
			expected.TraceID(),
			actual.TraceID())
	}

	if expected.SpanID() != actual.SpanID() {
		return fmt.Errorf("span SpanID doesn't match expected: %d, actual: %d",
			expected.SpanID(),
			actual.SpanID())
	}

	if expected.TraceState().AsRaw() != actual.TraceState().AsRaw() {
		return fmt.Errorf("span TraceState doesn't match expected: %s, actual: %s",
			expected.TraceState().AsRaw(),
			actual.TraceState().AsRaw())
	}

	if expected.ParentSpanID() != actual.ParentSpanID() {
		return fmt.Errorf("span ParentSpanID doesn't match expected: %d, actual: %d",
			expected.ParentSpanID(),
			actual.ParentSpanID())
	}

	if expected.Name() != actual.Name() {
		return fmt.Errorf("span Name doesn't match expected: %s, actual: %s",
			expected.Name(),
			actual.Name())
	}

	if expected.Kind() != actual.Kind() {
		return fmt.Errorf("span Kind doesn't match expected: %d, actual: %d",
			expected.Kind(),
			actual.Kind())
	}

	if expected.StartTimestamp() != actual.StartTimestamp() {
		return fmt.Errorf("span StartTimestamp doesn't match expected: %d, actual: %d",
			expected.StartTimestamp(),
			actual.StartTimestamp())
	}

	if expected.EndTimestamp() != actual.EndTimestamp() {
		return fmt.Errorf("span EndTimestamp doesn't match expected: %d, actual: %d",
			expected.EndTimestamp(),
			actual.EndTimestamp())
	}

	if !reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) {
		return fmt.Errorf("span Attributes doesn't match expected: %s, actual: %s",
			expected.Attributes().AsRaw(),
			actual.Attributes().AsRaw())
	}

	if expected.DroppedAttributesCount() != actual.DroppedAttributesCount() {
		return fmt.Errorf("span DroppedAttributesCount doesn't match expected: %d, actual: %d",
			expected.DroppedAttributesCount(),
			actual.DroppedAttributesCount())
	}

	if !reflect.DeepEqual(expected.Events(), actual.Events()) {
		return fmt.Errorf("span Events doesn't match expected: %v, actual: %v",
			expected.Events(),
			actual.Events())
	}

	if expected.DroppedEventsCount() != actual.DroppedEventsCount() {
		return fmt.Errorf("span DroppedEventsCount doesn't match expected: %d, actual: %d",
			expected.DroppedEventsCount(),
			actual.DroppedEventsCount())
	}

	if !reflect.DeepEqual(expected.Links(), actual.Links()) {
		return fmt.Errorf("span Links doesn't match expected: %v, actual: %v",
			expected.Links(),
			actual.Links())
	}

	if expected.DroppedLinksCount() != actual.DroppedLinksCount() {
		return fmt.Errorf("span DroppedLinksCount doesn't match expected: %d, actual: %d",
			expected.DroppedLinksCount(),
			actual.DroppedLinksCount())
	}

	if !reflect.DeepEqual(expected.Status(), actual.Status()) {
		return fmt.Errorf("span Status doesn't match expected: %v, actual: %v",
			expected.Status(),
			actual.Status())
	}

	return nil
}
