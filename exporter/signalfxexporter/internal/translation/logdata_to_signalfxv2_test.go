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

package translation

import (
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// This is basically the reverse of the test in the signalfxreceiver for
// converting from events to logs
func TestLogDataToSignalFxEvents(t *testing.T) {
	now := time.Now()
	msec := now.UnixNano() / 1e6

	// Put it on stack or heap so we can take a ref to it.
	userDefinedCat := sfxpb.EventCategory_USER_DEFINED

	buildDefaultSFxEvent := func() *sfxpb.Event {
		return &sfxpb.Event{
			EventType:  "shutdown",
			Timestamp:  msec,
			Category:   &userDefinedCat,
			Dimensions: buildNDimensions(4),
			Properties: mapToEventProps(map[string]interface{}{
				"env":      "prod",
				"isActive": true,
				"rack":     5,
				"temp":     40.5,
			}),
		}
	}

	buildDefaultLogs := func() plog.Logs {
		logs := plog.NewLogs()
		resourceLogs := logs.ResourceLogs()
		resourceLog := resourceLogs.AppendEmpty()
		resourceLog.Resource().Attributes().PutStr("k0", "should use ILL attr value instead")
		resourceLog.Resource().Attributes().PutStr("k3", "v3")
		resourceLog.Resource().Attributes().PutInt("k4", 123)

		ilLogs := resourceLog.ScopeLogs()
		logSlice := ilLogs.AppendEmpty().LogRecords()

		l := logSlice.AppendEmpty()
		l.SetTimestamp(pcommon.NewTimestampFromTime(now.Truncate(time.Millisecond)))
		attrs := l.Attributes()

		attrs.PutStr("k0", "v0")
		attrs.PutStr("k1", "v1")
		attrs.PutStr("k2", "v2")
		attrs.PutInt("com.splunk.signalfx.event_category", int64(sfxpb.EventCategory_USER_DEFINED))
		attrs.PutStr("com.splunk.signalfx.event_type", "shutdown")

		propMap := attrs.PutEmptyMap("com.splunk.signalfx.event_properties")
		propMap.PutStr("env", "prod")
		propMap.PutBool("isActive", true)
		propMap.PutInt("rack", 5)
		propMap.PutDouble("temp", 40.5)

		return logs
	}

	tests := []struct {
		name       string
		sfxEvents  []*sfxpb.Event
		logData    plog.Logs
		numDropped int
	}{
		{
			name:      "default",
			sfxEvents: []*sfxpb.Event{buildDefaultSFxEvent()},
			logData:   buildDefaultLogs(),
		},
		{
			name: "missing category",
			sfxEvents: func() []*sfxpb.Event {
				e := buildDefaultSFxEvent()
				e.Category = nil
				return []*sfxpb.Event{e}
			}(),
			logData: func() plog.Logs {
				logs := buildDefaultLogs()
				lrs := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
				lrs.At(0).Attributes().PutEmpty("com.splunk.signalfx.event_category")
				return logs
			}(),
		},
		{
			name: "missing event type",
			sfxEvents: func() []*sfxpb.Event {
				e := buildDefaultSFxEvent()
				e.EventType = "unknown"
				return []*sfxpb.Event{e}
			}(),
			logData: func() plog.Logs {
				logs := buildDefaultLogs()
				lrs := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
				lrs.At(0).Attributes().Remove("com.splunk.signalfx.event_type")
				return logs
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := tt.logData.ResourceLogs().At(0).Resource()
			logSlice := tt.logData.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
			events, dropped := LogRecordSliceToSignalFxV2(zap.NewNop(), logSlice, resource.Attributes())

			for k := range events {
				sort.Slice(events[k].Properties, func(i, j int) bool {
					return events[k].Properties[i].Key < events[k].Properties[j].Key
				})
				sort.Slice(events[k].Dimensions, func(i, j int) bool {
					return events[k].Dimensions[i].Key < events[k].Dimensions[j].Key
				})
			}
			assert.Equal(t, tt.sfxEvents, events)
			assert.Equal(t, tt.numDropped, dropped)
		})
	}
}

func mapToEventProps(m map[string]interface{}) []*sfxpb.Property {
	var out []*sfxpb.Property
	for k, v := range m {
		var pval sfxpb.PropertyValue

		switch t := v.(type) {
		case string:
			pval.StrValue = &t
		case int:
			asInt := int64(t)
			pval.IntValue = &asInt
		case int64:
			pval.IntValue = &t
		case bool:
			pval.BoolValue = &t
		case float64:
			pval.DoubleValue = &t
		default:
			panic(fmt.Sprintf("invalid type: %v", v))
		}

		out = append(out, &sfxpb.Property{
			Key:   k,
			Value: &pval,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out
}

func buildNDimensions(n uint) []*sfxpb.Dimension {
	d := make([]*sfxpb.Dimension, 0, n)
	for i := uint(0); i < n; i++ {
		idx := int(i)
		suffix := strconv.Itoa(idx)
		d = append(d, &sfxpb.Dimension{
			Key:   "k" + suffix,
			Value: "v" + suffix,
		})
	}
	return d
}
