// Copyright 2019 OpenTelemetry Authors
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

package honeycombexporter

import (
	"testing"
	"time"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
)

func TestSpanAttributesToMap(t *testing.T) {

	newSpanAttr := func(key string, value *tracepb.AttributeValue, count int32) *tracepb.Span_Attributes {
		return &tracepb.Span_Attributes{
			AttributeMap: map[string]*tracepb.AttributeValue{
				key: value,
			},
			DroppedAttributesCount: count,
		}
	}

	spanAttrs := []*tracepb.Span_Attributes{
		newSpanAttr("foo", &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_StringValue{
				StringValue: &tracepb.TruncatableString{Value: "bar"},
			},
		}, 0),
		newSpanAttr("foo", &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_IntValue{
				IntValue: 1234,
			},
		}, 0),
		newSpanAttr("foo", &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_BoolValue{
				BoolValue: true,
			},
		}, 0),
		newSpanAttr("foo", &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_DoubleValue{
				DoubleValue: 0.3145,
			},
		}, 0),
		nil,
		{
			AttributeMap:           nil,
			DroppedAttributesCount: 0,
		},
	}

	wantResults := []map[string]interface{}{
		{"foo": "bar"},
		{"foo": int64(1234)},
		{"foo": true},
		{"foo": 0.3145},
		{},
		{},
	}

	for i, attrs := range spanAttrs {
		got := spanAttributesToMap(attrs)
		want := wantResults[i]
		for k := range want {
			if interface{}(got[k]) != want[k] {
				t.Errorf("Got: %+v, Want: %+v", got[k], want[k])
			}
		}
		i++
	}
}

func TestTimestampToTime(t *testing.T) {
	var t1 time.Time
	emptyTime := timestampToTime(nil)
	if t1 != emptyTime {
		t.Errorf("Expected %+v, Got: %+v\n", t1, emptyTime)
	}

	t2 := time.Now()
	seconds := t2.UnixNano() / 1000000000
	nowTime := timestampToTime(&timestamp.Timestamp{
		Seconds: seconds,
		Nanos:   int32(t2.UnixNano() - (seconds * 1000000000)),
	})

	if !t2.Equal(nowTime) {
		t.Errorf("Expected %+v, Got %+v\n", t2, nowTime)
	}
}
