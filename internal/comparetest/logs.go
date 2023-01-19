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

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
)

// CompareLogs compares each part of two given Logs and returns
// an error if they don't match. The error describes what didn't match.
func CompareLogs(expected, actual plog.Logs, options ...LogsCompareOption) error {
	exp, act := plog.NewLogs(), plog.NewLogs()
	expected.CopyTo(exp)
	actual.CopyTo(act)

	for _, option := range options {
		option.applyOnLogs(exp, act)
	}

	expectedLogs, actualLogs := exp.ResourceLogs(), act.ResourceLogs()
	if expectedLogs.Len() != actualLogs.Len() {
		return fmt.Errorf("amount of ResourceLogs between Logs are not equal expected: %d, actual: %d",
			expectedLogs.Len(),
			actualLogs.Len())
	}

	numResources := expectedLogs.Len()

	// Keep track of matching resources so that each can only be matched once
	matchingResources := make(map[plog.ResourceLogs]plog.ResourceLogs, numResources)

	var errs error
	var outOfOrderErr error
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
				if e != a && outOfOrderErr == nil {
					outOfOrderErr = fmt.Errorf("ResourceLogs with attributes %v expected at index %d, "+
						"found a at index %d", er.Resource().Attributes().AsRaw(), e, a)
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected resource with attributes: %v", er.Resource().Attributes().AsRaw()))
		}
	}

	for i := 0; i < numResources; i++ {
		if _, ok := matchingResources[actualLogs.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("extra resource with attributes: %v", actualLogs.At(i).Resource().Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}

	if outOfOrderErr != nil {
		return outOfOrderErr
	}

	for ar, er := range matchingResources {
		if err := CompareResourceLogs(er, ar); err != nil {
			return err
		}
	}

	return errs
}

// CompareResourceLogs compares each part of two given ResourceLogs and returns
// an error if they don't match. The error describes what didn't match.
func CompareResourceLogs(expected, actual plog.ResourceLogs) error {
	exp, act := plog.NewResourceLogs(), plog.NewResourceLogs()
	expected.CopyTo(exp)
	actual.CopyTo(act)

	eilms := exp.ScopeLogs()
	ailms := act.ScopeLogs()

	if eilms.Len() != ailms.Len() {
		return fmt.Errorf("number of instrumentation libraries does not match expected: %d, actual: %d", eilms.Len(),
			ailms.Len())
	}

	// sort InstrumentationLibrary
	eilms.Sort(sortLogsInstrumentationLibrary)
	ailms.Sort(sortLogsInstrumentationLibrary)

	for i := 0; i < eilms.Len(); i++ {
		eilm, ailm := eilms.At(i), ailms.At(i)
		eil, ail := eilm.Scope(), ailm.Scope()

		if eil.Name() != ail.Name() {
			return fmt.Errorf("instrumentation library Name does not match expected: %s, actual: %s", eil.Name(), ail.Name())
		}
		if eil.Version() != ail.Version() {
			return fmt.Errorf("instrumentation library Version does not match expected: %s, actual: %s", eil.Version(), ail.Version())
		}
		if err := CompareLogRecordSlices(eilm.LogRecords(), ailm.LogRecords()); err != nil {
			return err
		}
	}
	return nil
}

// CompareLogRecordSlices compares each part of two given LogRecordSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareLogRecordSlices(expected, actual plog.LogRecordSlice) error {
	exp, act := plog.NewLogRecordSlice(), plog.NewLogRecordSlice()
	expected.CopyTo(exp)
	actual.CopyTo(act)

	if exp.Len() != act.Len() {
		return fmt.Errorf("number of log records does not match expected: %d, actual: %d", exp.Len(), act.Len())
	}

	exp.Sort(sortLogRecordSlice)
	act.Sort(sortLogRecordSlice)

	numLogRecords := exp.Len()

	// Keep track of matching records so that each record can only be matched once
	matchingLogRecords := make(map[plog.LogRecord]plog.LogRecord, numLogRecords)

	var errs error
	for e := 0; e < numLogRecords; e++ {
		elr := exp.At(e)
		var foundMatch bool
		for a := 0; a < numLogRecords; a++ {
			alr := act.At(a)
			if _, ok := matchingLogRecords[alr]; ok {
				continue
			}
			if reflect.DeepEqual(elr.Attributes().AsRaw(), alr.Attributes().AsRaw()) {
				foundMatch = true
				matchingLogRecords[alr] = elr
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("log missing expected resource with attributes: %v", elr.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numLogRecords; i++ {
		if _, ok := matchingLogRecords[act.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("log has extra record with attributes: %v", act.At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}

	for alr, elr := range matchingLogRecords {
		if err := CompareLogRecords(alr, elr); err != nil {
			return multierr.Combine(fmt.Errorf("log record with attributes: %v, does not match expected", alr.Attributes().AsRaw()), err)
		}
	}
	return nil
}

// CompareLogRecords compares each part of two given LogRecord and returns
// an error if they don't match. The error describes what didn't match.
func CompareLogRecords(expected, actual plog.LogRecord) error {
	if expected.Flags() != actual.Flags() {
		return fmt.Errorf("log record Flags doesn't match expected: %d, actual: %d",
			expected.Flags(),
			actual.Flags())
	}

	if expected.DroppedAttributesCount() != actual.DroppedAttributesCount() {
		return fmt.Errorf("log record DroppedAttributesCount doesn't match expected: %d, actual: %d",
			expected.DroppedAttributesCount(),
			actual.DroppedAttributesCount())
	}

	if expected.Timestamp() != actual.Timestamp() {
		return fmt.Errorf("log record Timestamp doesn't match expected: %d, actual: %d",
			expected.Timestamp(),
			actual.Timestamp())
	}

	if expected.ObservedTimestamp() != actual.ObservedTimestamp() {
		return fmt.Errorf("log record ObservedTimestamp doesn't match expected: %d, actual: %d",
			expected.ObservedTimestamp(),
			actual.ObservedTimestamp())
	}

	if expected.SeverityNumber() != actual.SeverityNumber() {
		return fmt.Errorf("log record SeverityNumber doesn't match expected: %d, actual: %d",
			expected.SeverityNumber(),
			actual.SeverityNumber())
	}

	if expected.SeverityText() != actual.SeverityText() {
		return fmt.Errorf("log record SeverityText doesn't match expected: %s, actual: %s",
			expected.SeverityText(),
			actual.SeverityText())
	}

	if expected.TraceID() != actual.TraceID() {
		return fmt.Errorf("log record TraceID doesn't match expected: %d, actual: %d",
			expected.TraceID(),
			actual.TraceID())
	}

	if expected.SpanID() != actual.SpanID() {
		return fmt.Errorf("log record SpanID doesn't match expected: %d, actual: %d",
			expected.SpanID(),
			actual.SpanID())
	}

	if !reflect.DeepEqual(expected.Body().AsRaw(), actual.Body().AsRaw()) {
		return fmt.Errorf("log record Body doesn't match expected: %s, actual: %s",
			expected.Body().AsString(),
			actual.Body().AsString())
	}

	return nil
}
