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

package comparetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/comparetest"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
)

// CompareTraces compares each part of two given Traces and returns
// an error if they don't match. The error describes what didn't match.
func CompareTraces(expected, actual ptrace.Traces, options ...TracesCompareOption) error {
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
							"found a at index %d", er.Resource().Attributes().AsRaw(), e, a))
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
			return err
		}
	}

	return nil
}

// CompareResourceSpans compares each part of two given ResourceSpans and returns
// an error if they don't match. The error describes what didn't match.
func CompareResourceSpans(expected, actual ptrace.ResourceSpans) error {
	eilms := expected.ScopeSpans()
	ailms := actual.ScopeSpans()

	if eilms.Len() != ailms.Len() {
		return fmt.Errorf("number of instrumentation libraries does not match expected: %d, actual: %d", eilms.Len(),
			ailms.Len())
	}

	for i := 0; i < eilms.Len(); i++ {
		eilm, ailm := eilms.At(i), ailms.At(i)
		eil, ail := eilm.Scope(), ailm.Scope()

		if eil.Name() != ail.Name() {
			return fmt.Errorf("instrumentation library Name does not match expected: %s, actual: %s", eil.Name(), ail.Name())
		}
		if eil.Version() != ail.Version() {
			return fmt.Errorf("instrumentation library Version does not match expected: %s, actual: %s", eil.Version(), ail.Version())
		}
		if err := CompareSpanSlices(eilm.Spans(), ailm.Spans()); err != nil {
			return err
		}
	}
	return nil
}

// CompareSpanSlices compares each part of two given SpanSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareSpanSlices(expected, actual ptrace.SpanSlice) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("number of spans does not match expected: %d, actual: %d", expected.Len(), actual.Len())
	}

	numSpans := expected.Len()

	// Keep track of matching spans so that each span can only be matched once
	matchingSpans := make(map[ptrace.Span]ptrace.Span, numSpans)

	var errs error
	var outOfOrderErrs error
	for e := 0; e < numSpans; e++ {
		elr := expected.At(e)
		var foundMatch bool
		for a := 0; a < numSpans; a++ {
			alr := actual.At(a)
			if _, ok := matchingSpans[alr]; ok {
				continue
			}
			if reflect.DeepEqual(elr.Attributes().AsRaw(), alr.Attributes().AsRaw()) {
				foundMatch = true
				matchingSpans[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf("span with attributes %v expected at index %d, "+
							"found a at index %d", elr.Attributes().AsRaw(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("span missing expected resource with attributes: %v", elr.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numSpans; i++ {
		if _, ok := matchingSpans[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("span has extra record with attributes: %v",
				actual.At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingSpans {
		if err := CompareSpans(alr, elr); err != nil {
			return multierr.Combine(fmt.Errorf("span with attributes: %v, does not match expected %v", alr.Attributes().AsRaw(), elr.Attributes().AsRaw()), err)
		}
	}
	return nil
}

// CompareSpans compares each part of two given Span and returns
// an error if they don't match. The error describes what didn't match.
func CompareSpans(expected, actual ptrace.Span) error {
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
