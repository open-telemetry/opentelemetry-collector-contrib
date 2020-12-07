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
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
)

// spanAttributesToMap converts an opencensus proto Span_Attributes object into a map
// of strings to generic types usable for sending events to honeycomb.
func spanAttributesToMap(spanAttrs pdata.AttributeMap) map[string]interface{} {
	var attrs = make(map[string]interface{}, spanAttrs.Len())

	spanAttrs.ForEach(func(key string, value pdata.AttributeValue) {
		switch value.Type() {
		case pdata.AttributeValueSTRING:
			attrs[key] = value.StringVal()
		case pdata.AttributeValueBOOL:
			attrs[key] = value.BoolVal()
		case pdata.AttributeValueINT:
			attrs[key] = value.IntVal()
		case pdata.AttributeValueDOUBLE:
			attrs[key] = value.DoubleVal()
		}
	})

	return attrs
}

// timestampToTime converts a protobuf timestamp into a time.Time.
func timestampToTime(ts pdata.TimestampUnixNano) (t time.Time) {
	if ts == 0 {
		return
	}
	return time.Unix(0, int64(ts)).UTC()
}

// getStatusCode returns the status code
func getStatusCode(status pdata.SpanStatus) int32 {
	return int32(status.Code())
}

// getStatusMessage returns the status message as a string
func getStatusMessage(status pdata.SpanStatus) string {
	if len(status.Message()) > 0 {
		return status.Message()
	}

	return status.Code().String()
}
