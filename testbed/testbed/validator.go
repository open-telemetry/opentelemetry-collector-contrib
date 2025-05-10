// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"fmt"
	"log"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TestCaseValidator defines the interface for validating and reporting test results.
type TestCaseValidator interface {
	// Validate executes validation routines and test assertions.
	Validate(tc *TestCase)
	// RecordResults updates the TestResultsSummary for the test suite with results of a single test.
	RecordResults(tc *TestCase)
}

type LogPresentValidator struct {
	LogBody string
	Present bool
}

func (v *LogPresentValidator) Validate(tc *TestCase) {
	logMsg := v.LogBody
	var successMsg, errorMsg string
	if v.Present {
		successMsg = fmt.Sprintf("Log '%s' found", logMsg)
		errorMsg = fmt.Sprintf("Log '%s' not found", logMsg)
	} else {
		errorMsg = fmt.Sprintf("Log '%s' found", logMsg)
		successMsg = fmt.Sprintf("Log '%s' not found", logMsg)
	}

	if assert.Equal(tc.t, tc.AgentLogsContains(logMsg), v.Present, errorMsg) {
		log.Print(successMsg)
	}
}

func (v *LogPresentValidator) RecordResults(tc *TestCase) {

	var result string
	if tc.t.Failed() {
		result = "FAIL"
	} else {
		result = "PASS"
	}

	// Remove "Test" prefix from test name.
	testName := tc.t.Name()[4:]

	tc.resultsSummary.Add(tc.t.Name(), &LogPresentResults{
		testName: testName,
		result:   result,
		duration: time.Since(tc.startTime),
	})
}

// PerfTestValidator implements TestCaseValidator for test suites using PerformanceResults for summarizing results.
type PerfTestValidator struct{}

func (v *PerfTestValidator) Validate(tc *TestCase) {
	if assert.EqualValues(tc.t,
		int64(tc.LoadGenerator.DataItemsSent()),
		int64(tc.MockBackend.DataItemsReceived()),
		"Received and sent counters do not match.") {
		log.Printf("Sent and received data matches.")
	}
}

func (v *PerfTestValidator) RecordResults(tc *TestCase) {
	rc := tc.agentProc.GetTotalConsumption()

	var result string
	if tc.t.Failed() {
		result = "FAIL"
	} else {
		result = "PASS"
	}

	// Remove "Test" prefix from test name.
	testName := tc.t.Name()[4:]

	tc.resultsSummary.Add(tc.t.Name(), &PerformanceTestResult{
		testName:          testName,
		result:            result,
		receivedSpanCount: tc.MockBackend.DataItemsReceived(),
		sentSpanCount:     tc.LoadGenerator.DataItemsSent(),
		duration:          time.Since(tc.startTime),
		cpuPercentageAvg:  rc.CPUPercentAvg,
		cpuPercentageMax:  rc.CPUPercentMax,
		ramMibAvg:         rc.RAMMiBAvg,
		ramMibMax:         rc.RAMMiBMax,
		errorCause:        tc.errorCause,
	})
}

// CorrectnessTestValidator implements TestCaseValidator for test suites using CorrectnessResults for summarizing results.
type CorrectnessTestValidator struct {
	dataProvider         DataProvider
	assertionFailures    []*TraceAssertionFailure
	ignoreSpanLinksAttrs bool
}

func NewCorrectTestValidator(senderName string, receiverName string, provider DataProvider) *CorrectnessTestValidator {
	// TODO: Fix Jaeger span links attributes and tracestate.
	return &CorrectnessTestValidator{
		dataProvider:         provider,
		ignoreSpanLinksAttrs: senderName == "jaeger" || receiverName == "jaeger",
	}
}

func (v *CorrectnessTestValidator) Validate(tc *TestCase) {
	if assert.EqualValues(tc.t,
		int64(tc.LoadGenerator.DataItemsSent())-int64(tc.LoadGenerator.PermanentErrors()),
		int64(tc.MockBackend.DataItemsReceived()),
		"Received and sent counters do not match.") {
		log.Printf("Sent and received data counters match.")
	}
	if len(tc.MockBackend.ReceivedTraces) > 0 {
		v.assertSentRecdTracingDataEqual(append(tc.MockBackend.ReceivedTraces, tc.MockBackend.DroppedTraces...))
	}
	assert.Empty(tc.t, v.assertionFailures, "There are span data mismatches.")
}

func (v *CorrectnessTestValidator) RecordResults(tc *TestCase) {
	var result string
	if tc.t.Failed() {
		result = "FAIL"
	} else {
		result = "PASS"
	}

	// Remove "Test" prefix from test name.
	testName := tc.t.Name()[4:]
	tc.resultsSummary.Add(tc.t.Name(), &CorrectnessTestResult{
		testName:                   testName,
		result:                     result,
		duration:                   time.Since(tc.startTime),
		receivedSpanCount:          tc.MockBackend.DataItemsReceived(),
		sentSpanCount:              tc.LoadGenerator.DataItemsSent(),
		traceAssertionFailureCount: uint64(len(v.assertionFailures)),
		traceAssertionFailures:     v.assertionFailures,
	})
}

func (v *CorrectnessTestValidator) assertSentRecdTracingDataEqual(tracesList []ptrace.Traces) {
	spansMap := make(map[string]ptrace.Span)
	// TODO: Remove this hack, and add a way to retrieve all sent data.
	if val, ok := v.dataProvider.(*goldenDataProvider); ok {
		populateSpansMap(spansMap, val.tracesGenerated)
	}

	for _, td := range tracesList {
		rss := td.ResourceSpans()
		for i := 0; i < rss.Len(); i++ {
			ilss := rss.At(i).ScopeSpans()
			for j := 0; j < ilss.Len(); j++ {
				spans := ilss.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					recdSpan := spans.At(k)
					sentSpan := spansMap[traceIDAndSpanIDToString(recdSpan.TraceID(), recdSpan.SpanID())]
					v.diffSpan(sentSpan, recdSpan)
				}
			}
		}
	}
}

func (v *CorrectnessTestValidator) diffSpan(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	v.diffSpanTraceID(sentSpan, recdSpan)
	v.diffSpanSpanID(sentSpan, recdSpan)
	v.diffSpanTraceState(sentSpan, recdSpan)
	v.diffSpanParentSpanID(sentSpan, recdSpan)
	v.diffSpanName(sentSpan, recdSpan)
	v.diffSpanKind(sentSpan, recdSpan)
	v.diffSpanTimestamps(sentSpan, recdSpan)
	v.diffSpanAttributes(sentSpan, recdSpan)
	v.diffSpanEvents(sentSpan, recdSpan)
	v.diffSpanLinks(sentSpan, recdSpan)
	v.diffSpanStatus(sentSpan, recdSpan)
}

func (v *CorrectnessTestValidator) diffSpanTraceID(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	if sentSpan.TraceID() != recdSpan.TraceID() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "TraceId",
			expectedValue: sentSpan.TraceID(),
			actualValue:   recdSpan.TraceID(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func (v *CorrectnessTestValidator) diffSpanSpanID(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	if sentSpan.SpanID() != recdSpan.SpanID() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "SpanId",
			expectedValue: sentSpan.SpanID(),
			actualValue:   recdSpan.SpanID(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func (v *CorrectnessTestValidator) diffSpanTraceState(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	if sentSpan.TraceState().AsRaw() != recdSpan.TraceState().AsRaw() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "TraceState",
			expectedValue: sentSpan.TraceState().AsRaw(),
			actualValue:   recdSpan.TraceState().AsRaw(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func (v *CorrectnessTestValidator) diffSpanParentSpanID(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	if sentSpan.ParentSpanID() != recdSpan.ParentSpanID() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "ParentSpanId",
			expectedValue: sentSpan.ParentSpanID(),
			actualValue:   recdSpan.ParentSpanID(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func (v *CorrectnessTestValidator) diffSpanName(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	// Because of https://github.com/openzipkin/zipkin-go/pull/166 compare lower cases.
	if !strings.EqualFold(sentSpan.Name(), recdSpan.Name()) {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "Name",
			expectedValue: sentSpan.Name(),
			actualValue:   recdSpan.Name,
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func (v *CorrectnessTestValidator) diffSpanKind(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	if sentSpan.Kind() != recdSpan.Kind() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "Kind",
			expectedValue: sentSpan.Kind,
			actualValue:   recdSpan.Kind,
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func (v *CorrectnessTestValidator) diffSpanTimestamps(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	if notWithinOneMillisecond(sentSpan.StartTimestamp(), recdSpan.StartTimestamp()) {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "StartTimestamp",
			expectedValue: sentSpan.StartTimestamp(),
			actualValue:   recdSpan.StartTimestamp(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
	if notWithinOneMillisecond(sentSpan.EndTimestamp(), recdSpan.EndTimestamp()) {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "EndTimestamp",
			expectedValue: sentSpan.EndTimestamp(),
			actualValue:   recdSpan.EndTimestamp(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func (v *CorrectnessTestValidator) diffSpanAttributes(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	if sentSpan.Attributes().Len() != recdSpan.Attributes().Len() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "Attributes",
			expectedValue: sentSpan.Attributes().Len(),
			actualValue:   recdSpan.Attributes().Len(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	} else {
		v.diffAttributeMap(sentSpan.Name(), sentSpan.Attributes(), recdSpan.Attributes(), "Attributes[%s]")
	}
	if sentSpan.DroppedAttributesCount() != recdSpan.DroppedAttributesCount() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "DroppedAttributesCount",
			expectedValue: sentSpan.DroppedAttributesCount(),
			actualValue:   recdSpan.DroppedAttributesCount(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func (v *CorrectnessTestValidator) diffSpanEvents(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	if sentSpan.Events().Len() != recdSpan.Events().Len() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "Events",
			expectedValue: sentSpan.Events().Len(),
			actualValue:   recdSpan.Events().Len(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	} else {
		sentEventMap := convertEventsSliceToMap(sentSpan.Events())
		recdEventMap := convertEventsSliceToMap(recdSpan.Events())
		for name, sentEvents := range sentEventMap {
			recdEvents, match := recdEventMap[name]
			if match {
				match = len(sentEvents) == len(recdEvents)
			}
			if !match {
				af := &TraceAssertionFailure{
					typeName:      "Span",
					dataComboName: sentSpan.Name(),
					fieldPath:     fmt.Sprintf("Events[%s]", name),
					expectedValue: len(sentEvents),
					actualValue:   len(recdEvents),
				}
				v.assertionFailures = append(v.assertionFailures, af)
			} else {
				for i, sentEvent := range sentEvents {
					recdEvent := recdEvents[i]
					if notWithinOneMillisecond(sentEvent.Timestamp(), recdEvent.Timestamp()) {
						af := &TraceAssertionFailure{
							typeName:      "Span",
							dataComboName: sentSpan.Name(),
							fieldPath:     fmt.Sprintf("Events[%s].TimeUnixNano", name),
							expectedValue: sentEvent.Timestamp(),
							actualValue:   recdEvent.Timestamp(),
						}
						v.assertionFailures = append(v.assertionFailures, af)
					}
					v.diffAttributeMap(sentSpan.Name(), sentEvent.Attributes(), recdEvent.Attributes(),
						"Events["+name+"].Attributes[%s]")
				}
			}
		}
	}
	if sentSpan.DroppedEventsCount() != recdSpan.DroppedEventsCount() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "DroppedEventsCount",
			expectedValue: sentSpan.DroppedEventsCount(),
			actualValue:   recdSpan.DroppedEventsCount(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func (v *CorrectnessTestValidator) diffSpanLinks(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	if sentSpan.Links().Len() != recdSpan.Links().Len() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "Links",
			expectedValue: sentSpan.Links().Len(),
			actualValue:   recdSpan.Links().Len(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	} else {
		recdLinksMap := convertLinksSliceToMap(recdSpan.Links())
		sentSpanLinks := sentSpan.Links()
		for i := 0; i < sentSpanLinks.Len(); i++ {
			sentLink := sentSpanLinks.At(i)
			recdLink, ok := recdLinksMap[traceIDAndSpanIDToString(sentLink.TraceID(), sentLink.SpanID())]
			if ok {
				if v.ignoreSpanLinksAttrs {
					return
				}
				if sentLink.TraceState().AsRaw() != recdLink.TraceState().AsRaw() {
					af := &TraceAssertionFailure{
						typeName:      "Span",
						dataComboName: sentSpan.Name(),
						fieldPath:     "Link.TraceState",
						expectedValue: sentLink,
						actualValue:   recdLink,
					}
					v.assertionFailures = append(v.assertionFailures, af)
				}
				v.diffAttributeMap(sentSpan.Name(), sentLink.Attributes(), recdLink.Attributes(),
					"Link.Attributes[%s]")
			} else {
				af := &TraceAssertionFailure{
					typeName:      "Span",
					dataComboName: sentSpan.Name(),
					fieldPath:     fmt.Sprintf("Links[%d]", i),
					expectedValue: sentLink,
					actualValue:   nil,
				}
				v.assertionFailures = append(v.assertionFailures, af)
			}

		}
	}
	if sentSpan.DroppedLinksCount() != recdSpan.DroppedLinksCount() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "DroppedLinksCount",
			expectedValue: sentSpan.DroppedLinksCount(),
			actualValue:   recdSpan.DroppedLinksCount(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func (v *CorrectnessTestValidator) diffSpanStatus(sentSpan ptrace.Span, recdSpan ptrace.Span) {
	if sentSpan.Status().Code() != recdSpan.Status().Code() {
		af := &TraceAssertionFailure{
			typeName:      "Span",
			dataComboName: sentSpan.Name(),
			fieldPath:     "Status.Code",
			expectedValue: sentSpan.Status().Code(),
			actualValue:   recdSpan.Status().Code(),
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func (v *CorrectnessTestValidator) diffAttributeMap(spanName string,
	sentAttrs pcommon.Map, recdAttrs pcommon.Map, fmtStr string) {
	sentAttrs.Range(func(sentKey string, sentVal pcommon.Value) bool {
		recdVal, ok := recdAttrs.Get(sentKey)
		if !ok {
			af := &TraceAssertionFailure{
				typeName:      "Span",
				dataComboName: spanName,
				fieldPath:     fmt.Sprintf(fmtStr, sentKey),
				expectedValue: sentVal,
				actualValue:   nil,
			}
			v.assertionFailures = append(v.assertionFailures, af)
			return true
		}
		switch sentVal.Type() {
		case pcommon.ValueTypeMap:
			v.compareKeyValueList(spanName, sentVal, recdVal, fmtStr, sentKey)
		default:
			v.compareSimpleValues(spanName, sentVal, recdVal, fmtStr, sentKey)
		}
		return true
	})
}

func (v *CorrectnessTestValidator) compareSimpleValues(spanName string, sentVal pcommon.Value, recdVal pcommon.Value,
	fmtStr string, attrKey string) {
	if reflect.DeepEqual(sentVal.AsRaw(), recdVal.AsRaw()) {
		sentStr := sentVal.AsString()
		recdStr := recdVal.AsString()
		if !strings.EqualFold(sentStr, recdStr) {
			af := &TraceAssertionFailure{
				typeName:      "Span",
				dataComboName: spanName,
				fieldPath:     fmt.Sprintf(fmtStr, attrKey),
				expectedValue: sentVal.AsString(),
				actualValue:   recdVal.AsString(),
			}
			v.assertionFailures = append(v.assertionFailures, af)
		}
	}
}

func (v *CorrectnessTestValidator) compareKeyValueList(
	spanName string, sentVal pcommon.Value, recdVal pcommon.Value, fmtStr string, attrKey string) {
	switch recdVal.Type() {
	case pcommon.ValueTypeMap:
		v.diffAttributeMap(spanName, sentVal.Map(), recdVal.Map(), fmtStr)
	case pcommon.ValueTypeStr:
		v.compareSimpleValues(spanName, sentVal, recdVal, fmtStr, attrKey)
	default:
		af := &TraceAssertionFailure{
			typeName:      "SimpleAttribute",
			dataComboName: spanName,
			fieldPath:     fmt.Sprintf(fmtStr, attrKey),
			expectedValue: sentVal,
			actualValue:   recdVal,
		}
		v.assertionFailures = append(v.assertionFailures, af)
	}
}

func convertEventsSliceToMap(events ptrace.SpanEventSlice) map[string][]ptrace.SpanEvent {
	eventMap := make(map[string][]ptrace.SpanEvent)
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)
		eventMap[event.Name()] = append(eventMap[event.Name()], event)
	}
	for _, eventList := range eventMap {
		sortEventsByTimestamp(eventList)
	}
	return eventMap
}

func sortEventsByTimestamp(eventList []ptrace.SpanEvent) {
	sort.SliceStable(eventList, func(i, j int) bool { return eventList[i].Timestamp() < eventList[j].Timestamp() })
}

func convertLinksSliceToMap(links ptrace.SpanLinkSlice) map[string]ptrace.SpanLink {
	linkMap := make(map[string]ptrace.SpanLink)
	for i := 0; i < links.Len(); i++ {
		link := links.At(i)
		linkMap[traceIDAndSpanIDToString(link.TraceID(), link.SpanID())] = link
	}
	return linkMap
}

func notWithinOneMillisecond(sentNs pcommon.Timestamp, recdNs pcommon.Timestamp) bool {
	var diff pcommon.Timestamp
	if sentNs > recdNs {
		diff = sentNs - recdNs
	} else {
		diff = recdNs - sentNs
	}
	return diff > 1100000
}

func populateSpansMap(spansMap map[string]ptrace.Span, tds []ptrace.Traces) {
	for _, td := range tds {
		rss := td.ResourceSpans()
		for i := 0; i < rss.Len(); i++ {
			ilss := rss.At(i).ScopeSpans()
			for j := 0; j < ilss.Len(); j++ {
				spans := ilss.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					span := spans.At(k)
					key := traceIDAndSpanIDToString(span.TraceID(), span.SpanID())
					spansMap[key] = span
				}
			}
		}
	}
}

func traceIDAndSpanIDToString(traceID pcommon.TraceID, spanID pcommon.SpanID) string {
	return fmt.Sprintf("%s-%s", traceID, spanID)
}

type CorrectnessLogTestValidator struct {
	dataProvider DataProvider
}

func NewCorrectnessLogTestValidator(provider DataProvider) *CorrectnessLogTestValidator {
	return &CorrectnessLogTestValidator{
		dataProvider: provider,
	}
}

func (c *CorrectnessLogTestValidator) Validate(tc *TestCase) {
	if dataProvider, ok := c.dataProvider.(*perfTestDataProvider); ok {
		logsReceived := tc.MockBackend.ReceivedLogs

		idsSent := make([][2]string, 0)
		idsReceived := make([][2]string, 0)

		for batch := 0; batch < int(dataProvider.traceIDSequence.Load()); batch++ {
			for idx := 0; idx < dataProvider.options.ItemsPerBatch; idx++ {
				idsSent = append(idsSent, [2]string{"batch_" + strconv.Itoa(batch), "item_" + strconv.Itoa(idx)})
			}
		}
		for _, log := range logsReceived {
			for i := 0; i < log.ResourceLogs().Len(); i++ {
				for j := 0; j < log.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
					s := log.ResourceLogs().At(i).ScopeLogs().At(j)
					for k := 0; k < s.LogRecords().Len(); k++ {
						logRecord := s.LogRecords().At(k)
						batchIndex, ok := logRecord.Attributes().Get("batch_index")
						require.True(tc.t, ok, "batch_index missing from attributes; use perfDataProvider")
						itemIndex, ok := logRecord.Attributes().Get("item_index")
						require.True(tc.t, ok, "item_index missing from attributes; use perfDataProvider")

						idsReceived = append(idsReceived, [2]string{batchIndex.Str(), itemIndex.Str()})
					}
				}
			}
		}

		assert.ElementsMatch(tc.t, idsSent, idsReceived)
	}
}

func (c *CorrectnessLogTestValidator) RecordResults(tc *TestCase) {
	rc := tc.agentProc.GetTotalConsumption()

	var result string
	if tc.t.Failed() {
		result = "FAIL"
	} else {
		result = "PASS"
	}

	// Remove "Test" prefix from test name.
	testName := tc.t.Name()[4:]

	tc.resultsSummary.Add(tc.t.Name(), &PerformanceTestResult{
		testName:          testName,
		result:            result,
		receivedSpanCount: tc.MockBackend.DataItemsReceived(),
		sentSpanCount:     tc.LoadGenerator.DataItemsSent(),
		duration:          time.Since(tc.startTime),
		cpuPercentageAvg:  rc.CPUPercentAvg,
		cpuPercentageMax:  rc.CPUPercentMax,
		ramMibAvg:         rc.RAMMiBAvg,
		ramMibMax:         rc.RAMMiBMax,
		errorCause:        tc.errorCause,
	})
}
