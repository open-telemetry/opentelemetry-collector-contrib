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

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

func Test_traceDataToSplunk(t *testing.T) {
	logger := zap.NewNop()
	ts := &timestamp.Timestamp{
		Nanos: 0,
	}

	tests := []struct {
		name                string
		traceDataFn         func() consumerdata.TraceData
		wantSplunkEvents    []*splunkEvent
		wantNumDroppedSpans int
	}{
		{
			name: "valid",
			traceDataFn: func() consumerdata.TraceData {
				return consumerdata.TraceData{
					Spans: []*v1.Span{
						makeSpan("myspan", ts),
					},
				}
			},
			wantSplunkEvents: []*splunkEvent{
				commonSplunkEvent("myspan", ts),
			},
			wantNumDroppedSpans: 0,
		},
		{
			name: "missing_start_ts",
			traceDataFn: func() consumerdata.TraceData {
				return consumerdata.TraceData{
					Spans: []*v1.Span{
						makeSpan("myspan", nil),
					},
				}
			},
			wantSplunkEvents:    []*splunkEvent{},
			wantNumDroppedSpans: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEvents, gotNumDroppedSpans := traceDataToSplunk(logger, tt.traceDataFn(), &Config{})
			assert.Equal(t, tt.wantNumDroppedSpans, gotNumDroppedSpans)
			assert.Equal(t, len(tt.wantSplunkEvents), len(gotEvents))
			for i, want := range tt.wantSplunkEvents {
				assert.EqualValues(t, want, gotEvents[i])
			}
			assert.Equal(t, tt.wantSplunkEvents, gotEvents)
		})
	}
}

func makeSpan(name string, ts *timestamp.Timestamp) *v1.Span {
	trunceableName := &v1.TruncatableString{
		Value: name,
	}
	return &v1.Span{
		Name:      trunceableName,
		StartTime: ts,
	}
}

func commonSplunkEvent(
	name string,
	ts *timestamp.Timestamp,
) *splunkEvent {
	trunceableName := &v1.TruncatableString{
		Value: name,
	}
	span := v1.Span{Name: trunceableName, StartTime: ts}
	return &splunkEvent{
		Time:  timestampToEpochMilliseconds(ts),
		Host:  "unknown",
		Event: &span,
	}
}
