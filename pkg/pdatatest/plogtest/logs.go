// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package plogtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
)

// CompareLogs compares each part of two given Logs and returns
// an error if they don't match. The error describes what didn't match.
func CompareLogs(expected, actual plog.Logs, options ...CompareLogsOption) error {
	exp, act := plog.NewLogs(), plog.NewLogs()
	expected.CopyTo(exp)
	actual.CopyTo(act)

	for _, option := range options {
		option.applyOnLogs(exp, act)
	}

	expectedLogs, actualLogs := exp.ResourceLogs(), act.ResourceLogs()
	if expectedLogs.Len() != actualLogs.Len() {
		return fmt.Errorf("number of resources doesn't match expected: %d, actual: %d",
			expectedLogs.Len(), actualLogs.Len())
	}

	numResources := expectedLogs.Len()

	// Keep track of matching resources so that each can only be matched once
	matchingResources := make(map[plog.ResourceLogs]plog.ResourceLogs, numResources)

	var errs error
	var outOfOrderErrs error
	for e := 0; e < numResources; e++ {
		er := expectedLogs.At(e)
		var foundMatch bool
		for a := 0; a < numResources; a++ {
			ar := actualLogs.At(a)
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
		if _, ok := matchingResources[actualLogs.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected resource: %v", actualLogs.At(i).Resource().Attributes().AsRaw()))
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
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareResourceLogs(er, ar)))
	}

	return errs
}

// CompareResourceLogs compares each part of two given ResourceLogs and returns
// an error if they don't match. The error describes what didn't match.
func CompareResourceLogs(expected, actual plog.ResourceLogs) error {
	errs := multierr.Combine(
		internal.CompareResource(expected.Resource(), actual.Resource()),
		internal.CompareSchemaURL(expected.SchemaUrl(), actual.SchemaUrl()),
	)

	esls := expected.ScopeLogs()
	asls := actual.ScopeLogs()

	if esls.Len() != asls.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of scopes doesn't match expected: %d, actual: %d", esls.Len(),
			asls.Len()))
		return errs
	}

	numScopeLogs := esls.Len()

	// Keep track of matching scope logs so that each record can only be matched once
	matchingScopeLogs := make(map[plog.ScopeLogs]plog.ScopeLogs, numScopeLogs)

	var outOfOrderErrs error
	for e := 0; e < numScopeLogs; e++ {
		esl := expected.ScopeLogs().At(e)
		var foundMatch bool
		for a := 0; a < numScopeLogs; a++ {
			asl := actual.ScopeLogs().At(a)
			if _, ok := matchingScopeLogs[asl]; ok {
				continue
			}
			if esl.Scope().Name() == asl.Scope().Name() {
				foundMatch = true
				matchingScopeLogs[asl] = esl
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf("scopes are out of order: scope %s expected at index %d, found at index %d",
							esl.Scope().Name(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected scope: %s", esl.Scope().Name()))
		}
	}

	for i := 0; i < numScopeLogs; i++ {
		if _, ok := matchingScopeLogs[actual.ScopeLogs().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected scope: %s", actual.ScopeLogs().At(i).Scope().Name()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for i := 0; i < esls.Len(); i++ {
		errPrefix := fmt.Sprintf(`scope "%s"`, esls.At(i).Scope().Name())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareScopeLogs(esls.At(i), asls.At(i))))
	}

	return errs
}

// CompareScopeLogs compares each part of two given LogRecordSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareScopeLogs(expected, actual plog.ScopeLogs) error {
	errs := multierr.Combine(
		internal.CompareInstrumentationScope(expected.Scope(), actual.Scope()),
		internal.CompareSchemaURL(expected.SchemaUrl(), actual.SchemaUrl()),
	)

	if expected.LogRecords().Len() != actual.LogRecords().Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of log records doesn't match expected: %d, actual: %d",
			expected.LogRecords().Len(), actual.LogRecords().Len()))
		return errs
	}

	numLogRecords := expected.LogRecords().Len()

	// Keep track of matching records so that each record can only be matched once
	matchingLogRecords := make(map[plog.LogRecord]plog.LogRecord, numLogRecords)

	var outOfOrderErrs error
	for e := 0; e < numLogRecords; e++ {
		elr := expected.LogRecords().At(e)
		var foundMatch bool
		for a := 0; a < numLogRecords; a++ {
			alr := actual.LogRecords().At(a)
			if _, ok := matchingLogRecords[alr]; ok {
				continue
			}
			if reflect.DeepEqual(elr.Attributes().AsRaw(), alr.Attributes().AsRaw()) {
				foundMatch = true
				matchingLogRecords[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`log records are out of order: log record "%v" expected at index %d, found at index %d`,
							elr.Attributes().AsRaw(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected log record: %v", elr.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numLogRecords; i++ {
		if _, ok := matchingLogRecords[actual.LogRecords().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected log record: %v",
				actual.LogRecords().At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingLogRecords {
		errPrefix := fmt.Sprintf(`log record "%v"`, elr.Attributes().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareLogRecord(alr, elr)))
	}
	return errs
}

// CompareLogRecord compares each part of two given LogRecord and returns
// an error if they don't match. The error describes what didn't match.
func CompareLogRecord(expected, actual plog.LogRecord) error {
	errs := multierr.Combine(
		internal.CompareAttributes(expected.Attributes(), actual.Attributes()),
		internal.CompareDroppedAttributesCount(expected.DroppedAttributesCount(), actual.DroppedAttributesCount()),
	)

	if expected.Flags() != actual.Flags() {
		errs = multierr.Append(errs, fmt.Errorf("flags doesn't match expected: %d, actual: %d",
			expected.Flags(), actual.Flags()))
	}

	if expected.Timestamp() != actual.Timestamp() {
		errs = multierr.Append(errs, fmt.Errorf("timestamp doesn't match expected: %d, actual: %d",
			expected.Timestamp(), actual.Timestamp()))
	}

	if expected.ObservedTimestamp() != actual.ObservedTimestamp() {
		errs = multierr.Append(errs, fmt.Errorf("observed timestamp doesn't match expected: %d, actual: %d",
			expected.ObservedTimestamp(), actual.ObservedTimestamp()))
	}

	if expected.SeverityNumber() != actual.SeverityNumber() {
		errs = multierr.Append(errs, fmt.Errorf("severity number doesn't match expected: %s, actual: %s",
			expected.SeverityNumber(), actual.SeverityNumber()))
	}

	if expected.SeverityText() != actual.SeverityText() {
		errs = multierr.Append(errs, fmt.Errorf("severity text doesn't match expected: %s, actual: %s",
			expected.SeverityText(), actual.SeverityText()))
	}

	if expected.TraceID() != actual.TraceID() {
		errs = multierr.Append(errs, fmt.Errorf("trace ID doesn't match expected: %d, actual: %d",
			expected.TraceID(), actual.TraceID()))
	}

	if expected.SpanID() != actual.SpanID() {
		errs = multierr.Append(errs, fmt.Errorf("span ID doesn't match expected: %d, actual: %d",
			expected.SpanID(), actual.SpanID()))
	}

	if !reflect.DeepEqual(expected.Body().AsRaw(), actual.Body().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("body doesn't match expected: %s, actual: %s",
			expected.Body().AsString(), actual.Body().AsString()))
	}

	return errs
}
