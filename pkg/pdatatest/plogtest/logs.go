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

package plogtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
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
		return fmt.Errorf("amount of ResourceLogs between Logs are not equal expected: %d, actual: %d",
			expectedLogs.Len(),
			actualLogs.Len())
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
						fmt.Errorf("ResourceLogs with attributes %v expected at index %d, "+
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
		if _, ok := matchingResources[actualLogs.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("extra resource with attributes: %v", actualLogs.At(i).Resource().Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for ar, er := range matchingResources {
		if err := CompareResourceLogs(er, ar); err != nil {
			return multierr.Combine(fmt.Errorf("ResourceLogs with attributes %v does not match expected",
				ar.Resource().Attributes().AsRaw()), err)
		}
	}

	return errs
}

// CompareResourceLogs compares each part of two given ResourceLogs and returns
// an error if they don't match. The error describes what didn't match.
func CompareResourceLogs(expected, actual plog.ResourceLogs) error {
	if !reflect.DeepEqual(expected.Resource().Attributes().AsRaw(), actual.Resource().Attributes().AsRaw()) {
		return fmt.Errorf("resource attributes do not match expected: %v, actual: %v",
			expected.Resource().Attributes().AsRaw(), actual.Resource().Attributes().AsRaw())
	}

	esls := expected.ScopeLogs()
	asls := actual.ScopeLogs()

	if esls.Len() != asls.Len() {
		return fmt.Errorf("number of scope logs does not match expected: %d, actual: %d", esls.Len(),
			asls.Len())
	}

	numScopeLogs := esls.Len()

	// Keep track of matching scope logs so that each record can only be matched once
	matchingScopeLogs := make(map[plog.ScopeLogs]plog.ScopeLogs, numScopeLogs)

	var errs error
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
						fmt.Errorf("ScopeLogs with scope name %s expected at index %d, found at index %d",
							esl.Scope().Name(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing ScopeLogs with scope name: %s", esl.Scope().Name()))
		}
	}

	for i := 0; i < numScopeLogs; i++ {
		if _, ok := matchingScopeLogs[actual.ScopeLogs().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected ScopeLogs with scope name: %s",
				actual.ScopeLogs().At(i).Scope().Name()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for i := 0; i < esls.Len(); i++ {
		if err := CompareScopeLogs(esls.At(i), asls.At(i)); err != nil {
			return multierr.Combine(fmt.Errorf(`ScopeLogs with scope name "%s" do not match expected`,
				esls.At(i).Scope().Name()), err)
		}
	}
	return nil
}

// CompareScopeLogs compares each part of two given LogRecordSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareScopeLogs(expected, actual plog.ScopeLogs) error {
	if expected.Scope().Name() != actual.Scope().Name() {
		return fmt.Errorf("scope name does not match expected: %s, actual: %s",
			expected.Scope().Name(), actual.Scope().Name())
	}
	if expected.Scope().Version() != actual.Scope().Version() {
		return fmt.Errorf("scope version does not match expected: %s, actual: %s",
			expected.Scope().Version(), actual.Scope().Version())
	}

	if expected.LogRecords().Len() != actual.LogRecords().Len() {
		return fmt.Errorf("number of log records does not match expected: %d, actual: %d",
			expected.LogRecords().Len(), actual.LogRecords().Len())
	}

	numLogRecords := expected.LogRecords().Len()

	// Keep track of matching records so that each record can only be matched once
	matchingLogRecords := make(map[plog.LogRecord]plog.LogRecord, numLogRecords)

	var errs error
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
						fmt.Errorf("LogRecord with attributes %v expected at index %d, "+
							"found at index %d", elr.Attributes().AsRaw(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("log missing expected resource with attributes: %v", elr.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numLogRecords; i++ {
		if _, ok := matchingLogRecords[actual.LogRecords().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("log has extra record with attributes: %v",
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
		if err := CompareLogRecord(alr, elr); err != nil {
			return multierr.Combine(fmt.Errorf("log record with attributes %v does not match expected", alr.Attributes().AsRaw()), err)
		}
	}
	return nil
}

// CompareLogRecord compares each part of two given LogRecord and returns
// an error if they don't match. The error describes what didn't match.
func CompareLogRecord(expected, actual plog.LogRecord) error {
	if !reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) {
		return fmt.Errorf("log record attributes do not match expected: %v, actual: %v",
			expected.Attributes().AsRaw(), actual.Attributes().AsRaw())
	}

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
		return fmt.Errorf("log record SeverityNumber doesn't match expected: %s, actual: %s",
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
