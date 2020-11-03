// Copyright 2020, OpenTelemetry Authors
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

package splunkhecexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func Test_traceDataToSplunk(t *testing.T) {
	logger := zap.NewNop()
	ts := pdata.TimestampUnixNano(123)

	tests := []struct {
		name                string
		traceDataFn         func() pdata.Span
		wantSplunkEvents    []*splunk.Event
		wantNumDroppedSpans int
	}{
		{
			name: "valid",
			traceDataFn: func() pdata.Span {
				return makeSpan("myspan", &ts)
			},
			wantSplunkEvents: []*splunk.Event{
				commonSplunkEvent("myspan", ts),
			},
			wantNumDroppedSpans: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traces := pdata.NewTraces()
			rs := pdata.NewResourceSpans()
			rs.InitEmpty()
			ils := pdata.NewInstrumentationLibrarySpans()
			ils.InitEmpty()
			ils.Spans().Append(tt.traceDataFn())
			rs.InstrumentationLibrarySpans().Append(ils)
			traces.ResourceSpans().Append(rs)
			gotEvents, gotNumDroppedSpans := traceDataToSplunk(logger, traces, &Config{})
			assert.Equal(t, tt.wantNumDroppedSpans, gotNumDroppedSpans)
			require.Equal(t, len(tt.wantSplunkEvents), len(gotEvents))
			for i, want := range tt.wantSplunkEvents {
				assert.EqualValues(t, want, gotEvents[i])
			}
			assert.Equal(t, tt.wantSplunkEvents, gotEvents)
		})
	}
}

func makeSpan(name string, ts *pdata.TimestampUnixNano) pdata.Span {
	span := pdata.NewSpan()
	span.InitEmpty()
	span.SetName(name)
	if ts != nil {
		span.SetStartTime(*ts)
	}
	return span
}

func commonSplunkEvent(
	name string,
	ts pdata.TimestampUnixNano,
) *splunk.Event {
	return &splunk.Event{
		Time: timestampToSecondsWithMillisecondPrecision(ts),
		Host: "unknown",
		Event: HecSpan{Name: name, StartTime: ts,
			TraceID:    "",
			SpanID:     "",
			ParentSpan: "",
			Attributes: map[string]interface{}{},
			EndTime:    0x0,
			Kind:       "SPAN_KIND_UNSPECIFIED",
			Status:     HecSpanStatus{Message: "", Code: ""},
			Events:     []HecEvent{},
			Links:      []HecLink{}},
	}
}
