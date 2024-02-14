// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	errs := multierr.Combine(
		internal.CompareResource(expected.Resource(), actual.Resource()),
		internal.CompareSchemaURL(expected.SchemaUrl(), actual.SchemaUrl()),
	)

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
	errs := multierr.Combine(
		internal.CompareInstrumentationScope(expected.Scope(), actual.Scope()),
		internal.CompareSchemaURL(expected.SchemaUrl(), actual.SchemaUrl()),
	)

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
	errs := multierr.Combine(
		internal.CompareAttributes(expected.Attributes(), actual.Attributes()),
		internal.CompareDroppedAttributesCount(expected.DroppedAttributesCount(), actual.DroppedAttributesCount()),
	)

	if expected.TraceID() != actual.TraceID() {
		errs = multierr.Append(errs, fmt.Errorf("trace ID doesn't match expected: %s, actual: %s",
			expected.TraceID(), actual.TraceID()))
	}

	if expected.SpanID() != actual.SpanID() {
		errs = multierr.Append(errs, fmt.Errorf("span ID doesn't match expected: %s, actual: %s",
			expected.SpanID(), actual.SpanID()))
	}

	if expected.TraceState().AsRaw() != actual.TraceState().AsRaw() {
		errs = multierr.Append(errs, fmt.Errorf("trace state doesn't match expected: %s, actual: %s",
			expected.TraceState().AsRaw(), actual.TraceState().AsRaw()))
	}

	if expected.ParentSpanID() != actual.ParentSpanID() {
		errs = multierr.Append(errs, fmt.Errorf("parent span ID doesn't match expected: %s, actual: %s",
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

	errs = multierr.Append(errs, compareSpanEventSlice(expected.Events(), actual.Events()))

	if expected.DroppedEventsCount() != actual.DroppedEventsCount() {
		errs = multierr.Append(errs, fmt.Errorf("dropped events count doesn't match expected: %d, actual: %d",
			expected.DroppedEventsCount(), actual.DroppedEventsCount()))
	}

	errs = multierr.Append(errs, compareSpanLinkSlice(expected.Links(), actual.Links()))

	if expected.DroppedLinksCount() != actual.DroppedLinksCount() {
		errs = multierr.Append(errs, fmt.Errorf("dropped links count doesn't match expected: %d, actual: %d",
			expected.DroppedLinksCount(), actual.DroppedLinksCount()))
	}

	if expected.Status().Code() != actual.Status().Code() {
		errs = multierr.Append(errs, fmt.Errorf("status code doesn't match expected: %v, actual: %v",
			expected.Status().Code(), actual.Status().Code()))
	}

	if expected.Status().Message() != actual.Status().Message() {
		errs = multierr.Append(errs, fmt.Errorf("status message doesn't match expected: %v, actual: %v",
			expected.Status().Message(), actual.Status().Message()))
	}

	return errs
}

// compareSpanEventSlice compares each part of two given SpanEventSlice and returns
// an error if they don't match. The error describes what didn't match.
func compareSpanEventSlice(expected, actual ptrace.SpanEventSlice) (errs error) {
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of events doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numSpanEvents := expected.Len()

	// Keep track of matching span events so that each span event can only be matched once
	matchingSpanEvents := make(map[ptrace.SpanEvent]ptrace.SpanEvent, numSpanEvents)

	var outOfOrderErrs error
	for e := 0; e < numSpanEvents; e++ {
		ee := expected.At(e)
		var foundMatch bool
		for a := 0; a < numSpanEvents; a++ {
			ae := actual.At(a)
			if _, ok := matchingSpanEvents[ae]; ok {
				continue
			}
			if ee.Name() == ae.Name() {
				foundMatch = true
				matchingSpanEvents[ae] = ee
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`span events are out of order: span event "%s" expected at index %d, found at index %d`,
							ee.Name(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected span event: %s", ee.Name()))
		}
	}

	for i := 0; i < numSpanEvents; i++ {
		if _, ok := matchingSpanEvents[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected span event: %s", actual.At(i).Name()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for ae, ee := range matchingSpanEvents {
		errs = multierr.Append(errs, internal.AddErrPrefix(fmt.Sprintf(`span event "%s"`, ee.Name()), CompareSpanEvent(ae, ee)))
	}

	return errs
}

// CompareSpanEvent compares each part of two given SpanEvent and returns
// an error if they don't match. The error describes what didn't match.
func CompareSpanEvent(expected, actual ptrace.SpanEvent) error {
	errs := multierr.Combine(
		internal.CompareAttributes(expected.Attributes(), actual.Attributes()),
		internal.CompareDroppedAttributesCount(expected.DroppedAttributesCount(), actual.DroppedAttributesCount()),
	)

	if expected.Name() != actual.Name() {
		errs = multierr.Append(errs, fmt.Errorf("name doesn't match expected: %s, actual: %s",
			expected.Name(), actual.Name()))
	}

	if expected.Timestamp() != actual.Timestamp() {
		errs = multierr.Append(errs, fmt.Errorf("timestamp doesn't match expected: %d, actual: %d",
			expected.Timestamp(), actual.Timestamp()))
	}

	return errs
}

// compareSpanLinkSlice compares each part of two given SpanLinkSlice and returns
// an error if they don't match. The error describes what didn't match.
func compareSpanLinkSlice(expected, actual ptrace.SpanLinkSlice) (errs error) {
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of span links doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numSpanLinks := expected.Len()

	// Keep track of matching span links so that each span link can only be matched once
	matchingSpanLinks := make(map[ptrace.SpanLink]ptrace.SpanLink, numSpanLinks)

	var outOfOrderErrs error
	for e := 0; e < numSpanLinks; e++ {
		el := expected.At(e)
		var foundMatch bool
		for a := 0; a < numSpanLinks; a++ {
			al := actual.At(a)
			if _, ok := matchingSpanLinks[al]; ok {
				continue
			}
			if el.SpanID() == al.SpanID() {
				foundMatch = true
				matchingSpanLinks[al] = el
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`span links are out of order: span link "%s" expected at index %d, found at index %d`,
							el.SpanID(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected span link: %s", el.SpanID()))
		}
	}

	for i := 0; i < numSpanLinks; i++ {
		if _, ok := matchingSpanLinks[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected span link: %s", actual.At(i).SpanID()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for al, el := range matchingSpanLinks {
		errs = multierr.Append(errs, internal.AddErrPrefix(fmt.Sprintf(`span link "%s"`, el.SpanID()), CompareSpanLink(al, el)))
	}

	return errs
}

// CompareSpanLink compares each part of two given SpanLink and returns
// an error if they don't match. The error describes what didn't match.
func CompareSpanLink(expected, actual ptrace.SpanLink) error {
	errs := multierr.Combine(
		internal.CompareAttributes(expected.Attributes(), actual.Attributes()),
		internal.CompareDroppedAttributesCount(expected.DroppedAttributesCount(), actual.DroppedAttributesCount()),
	)

	if expected.TraceID() != actual.TraceID() {
		errs = multierr.Append(errs, fmt.Errorf("trace ID doesn't match expected: %s, actual: %s",
			expected.TraceID(), actual.TraceID()))
	}

	if expected.SpanID() != actual.SpanID() {
		errs = multierr.Append(errs, fmt.Errorf("span ID doesn't match expected: %s, actual: %s",
			expected.SpanID(), actual.SpanID()))
	}

	if expected.TraceState().AsRaw() != actual.TraceState().AsRaw() {
		errs = multierr.Append(errs, fmt.Errorf("trace state doesn't match expected: %s, actual: %s",
			expected.TraceState().AsRaw(), actual.TraceState().AsRaw()))
	}

	return errs
}
