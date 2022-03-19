// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package skywalkingreceiver

import (
	"strconv"
	"testing"

	"go.opentelemetry.io/collector/model/pdata"
	agentV3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"

	"github.com/stretchr/testify/assert"
)

func TestSetInternalSpanStatus(t *testing.T) {
	tests := []struct {
		name   string
		swSpan *agentV3.SpanObject
		dest   pdata.SpanStatus
		code   pdata.StatusCode
	}{
		{
			name: "StatusCodeError",
			swSpan: &agentV3.SpanObject{
				IsError: true,
			},
			dest: generateTracesOneEmptyResourceSpans().Status(),
			code: pdata.StatusCodeError,
		},
		{
			name: "StatusCodeOk",
			swSpan: &agentV3.SpanObject{
				IsError: false,
			},
			dest: generateTracesOneEmptyResourceSpans().Status(),
			code: pdata.StatusCodeOk,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setInternalSpanStatus(test.swSpan, test.dest)
			assert.Equal(t, test.code, test.dest.Code())
		})
	}
}

func TestSwKvPairsToInternalAttributes(t *testing.T) {
	tests := []struct {
		name   string
		swSpan *agentV3.SegmentObject
		dest   pdata.Span
	}{
		{
			name:   "mock-sw-swgment-1",
			swSpan: mockGrpcTraceSegment(1),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
		{
			name:   "mock-sw-swgment-2",
			swSpan: mockGrpcTraceSegment(2),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			swKvPairsToInternalAttributes(test.swSpan.GetSpans()[0].Tags, test.dest.Attributes())
			assert.Equal(t, test.dest.Attributes().Len(), len(test.swSpan.GetSpans()[0].Tags))
			for _, tag := range test.swSpan.GetSpans()[0].Tags {
				value, _ := test.dest.Attributes().Get(tag.Key)
				assert.Equal(t, tag.Value, value.AsString())
			}
		})
	}
}

func TestSwProtoToTraces(t *testing.T) {
	tests := []struct {
		name   string
		swSpan *agentV3.SegmentObject
		dest   pdata.Traces
		code   pdata.StatusCode
	}{
		{
			name:   "mock-sw-swgment-1",
			swSpan: mockGrpcTraceSegment(1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			td := SkywalkingToTraces(test.swSpan)
			assert.Equal(t, 1, td.ResourceSpans().Len())
			assert.Equal(t, 2, td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().Len())
		})
	}
}

func TestSwReferencesToSpanLinks(t *testing.T) {
	tests := []struct {
		name   string
		swSpan *agentV3.SegmentObject
		dest   pdata.Span
	}{
		{
			name:   "mock-sw-swgment-1",
			swSpan: mockGrpcTraceSegment(1),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
		{
			name:   "mock-sw-swgment-2",
			swSpan: mockGrpcTraceSegment(2),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			swReferencesToSpanLinks(test.swSpan.GetSpans()[0].Refs, test.dest.Links())
			assert.Equal(t, 1, test.dest.Links().Len())
		})
	}
}

func TestSwLogsToSpanEvents(t *testing.T) {
	tests := []struct {
		name   string
		swSpan *agentV3.SegmentObject
		dest   pdata.Span
	}{
		{
			name:   "mock-sw-swgment-0",
			swSpan: mockGrpcTraceSegment(0),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
		{
			name:   "mock-sw-swgment-1",
			swSpan: mockGrpcTraceSegment(1),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seq := strconv.Itoa(index)
			swLogsToSpanEvents(test.swSpan.GetSpans()[0].Logs, test.dest.Events())
			assert.Equal(t, 1, test.dest.Events().Len())
			assert.Equal(t, "logs", test.dest.Events().At(0).Name())
			logValue, _ := test.dest.Events().At(0).Attributes().Get("log-key" + seq)
			assert.Equal(t, "log-value"+seq, logValue.AsString())
		})
	}
}
func generateTracesOneEmptyResourceSpans() pdata.Span {
	td := pdata.NewTraces()
	resourceSpan := td.ResourceSpans().AppendEmpty()
	il := resourceSpan.InstrumentationLibrarySpans().AppendEmpty()
	il.Spans().AppendEmpty()
	return il.Spans().At(0)
}
