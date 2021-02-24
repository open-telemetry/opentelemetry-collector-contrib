// Copyright OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/consumer/pdata"
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
			Dimensions: buildNDimensions(3),
			Properties: mapToEventProps(map[string]interface{}{
				"env":      "prod",
				"isActive": true,
				"rack":     5,
				"temp":     40.5,
			}),
		}
	}

	buildDefaultLogs := func() pdata.LogSlice {
		logSlice := pdata.NewLogSlice()

		logSlice.Resize(1)
		l := logSlice.At(0)

		l.SetName("shutdown")
		l.SetTimestamp(pdata.TimestampUnixNano(now.Truncate(time.Millisecond).UnixNano()))
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
		})
		propMap.Sort()
		attrs.Insert("com.splunk.signalfx.event_properties", propMapVal)
		attrs.Insert("com.splunk.signalfx.event_category", pdata.NewAttributeValueInt(int64(sfxpb.EventCategory_USER_DEFINED)))

		l.Attributes().Sort()

		return logSlice
	}

	tests := []struct {
		name       string
		sfxEvents  []*sfxpb.Event
		logData    pdata.LogSlice
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
			logData: func() pdata.LogSlice {
				lrs := buildDefaultLogs()
				lrs.At(0).Attributes().Upsert("com.splunk.signalfx.event_category", pdata.NewAttributeValueNull())
				return lrs
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, dropped := LogSliceToSignalFxV2(zap.NewNop(), tt.logData)
			for i := 0; i < tt.logData.Len(); i++ {
				tt.logData.At(i).Attributes().Sort()
			}

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
