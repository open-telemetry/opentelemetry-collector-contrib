// Copyright 2019, OpenTelemetry Authors
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

package signalfxreceiver

import (
	"fmt"
	"sort"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestSignalFxV2EventsToLogData(t *testing.T) {
	now := time.Now()
	msec := now.UnixNano() / 1e6

	// Put it on stack or heap so we can take a ref to it.
	userDefinedCat := sfxpb.EventCategory_USER_DEFINED

	buildDefaultSFxEvent := func() *sfxpb.Event {
		return &sfxpb.Event{
			EventType:  "shutdown",
			Timestamp:  msec,
			Category:   &userDefinedCat,
			Dimensions: buildNDimensions(3),
			Properties: mapToEventProps(map[string]interface{}{
				"env":      "prod",
				"isActive": true,
				"rack":     5,
				"temp":     40.5,
				"nullProp": nil,
			}),
		}
	}

	buildDefaultLogs := func() pdata.LogSlice {
		logSlice := pdata.NewLogSlice()
		l := logSlice.AppendEmpty()
		l.SetName("shutdown")
		l.SetTimestamp(pdata.NewTimestampFromTime(now.Truncate(time.Millisecond)))
		attrs := l.Attributes()

		attrs.InitFromMap(map[string]pdata.AttributeValue{
			"k0": pdata.NewAttributeValueString("v0"),
			"k1": pdata.NewAttributeValueString("v1"),
			"k2": pdata.NewAttributeValueString("v2"),
		})

		propMapVal := pdata.NewAttributeValueMap()
		propMap := propMapVal.MapVal()
		propMap.InitFromMap(map[string]pdata.AttributeValue{
			"env":      pdata.NewAttributeValueString("prod"),
			"isActive": pdata.NewAttributeValueBool(true),
			"rack":     pdata.NewAttributeValueInt(5),
			"temp":     pdata.NewAttributeValueDouble(40.5),
			"nullProp": pdata.NewAttributeValueEmpty(),
		})
		propMap.Sort()
		attrs.Insert("com.splunk.signalfx.event_properties", propMapVal)
		attrs.Insert("com.splunk.signalfx.event_category", pdata.NewAttributeValueInt(int64(sfxpb.EventCategory_USER_DEFINED)))

		l.Attributes().Sort()
		return logSlice
	}

	tests := []struct {
		name      string
		sfxEvents []*sfxpb.Event
		expected  pdata.LogSlice
	}{
		{
			name:      "default",
			sfxEvents: []*sfxpb.Event{buildDefaultSFxEvent()},
			expected:  buildDefaultLogs(),
		},
		{
			name: "missing category",
			sfxEvents: func() []*sfxpb.Event {
				e := buildDefaultSFxEvent()
				e.Category = nil
				return []*sfxpb.Event{e}
			}(),
			expected: func() pdata.LogSlice {
				lrs := buildDefaultLogs()
				lrs.At(0).Attributes().Upsert("com.splunk.signalfx.event_category", pdata.NewAttributeValueEmpty())
				return lrs
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lrs := pdata.NewLogSlice()
			signalFxV2EventsToLogRecords(tt.sfxEvents, lrs)
			for i := 0; i < lrs.Len(); i++ {
				lrs.At(i).Attributes().Sort()
			}
			assert.Equal(t, tt.expected, lrs)
		})
	}
}

func mapToEventProps(m map[string]interface{}) []*sfxpb.Property {
	var out []*sfxpb.Property
	for k, v := range m {
		var pval sfxpb.PropertyValue

		switch t := v.(type) {
		case nil:
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
