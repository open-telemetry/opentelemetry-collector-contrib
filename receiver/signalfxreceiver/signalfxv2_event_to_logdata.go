// Copyright -c Google LLC
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

package signalfxreceiver

import (
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

// SignalFxV2ToMetricsData converts SignalFx proto data points to
// consumerdata.MetricsData. Returning the converted data and the number of
// dropped time series.
func SignalFxV2EventsToLogRecords(
	logger *zap.Logger,
	events []*sfxpb.Event,
) pdata.LogSlice {
	lrs := pdata.NewLogSlice()

	for _, event := range events {
		lr := pdata.NewLogRecord()
		lr.InitEmpty()

		// The EventType field is the most logical "name" of the event.
		lr.SetName(event.EventType)

		// SignalFx timestamps are in millis so convert to nanos by multiplying
		// by 1 million.
		lr.SetTimestamp(pdata.TimestampUnixNano(event.Timestamp * 1e6))

		attrs := lr.Attributes()

		if event.Category != nil {
			attrs.InsertInt(splunk.SFxEventCategoryKey, int64(*event.Category))
		} else {
			// This gives us an unambiguous way of determining that a log record
			// represents a SignalFx event, even if category is missing from the
			// event.
			attrs.InsertNull(splunk.SFxEventCategoryKey)
		}

		for _, dim := range event.Dimensions {
			attrs.InsertString(dim.Key, dim.Value)
		}

		if len(event.Properties) > 0 {
			propMap := pdata.NewAttributeMap()
			for _, prop := range event.Properties {
				// No way to tell what value type is without testing each
				// individually.
				if prop.Value.StrValue != nil {
					propMap.InsertString(prop.Key, prop.Value.GetStrValue())
				} else if prop.Value.IntValue != nil {
					propMap.InsertInt(prop.Key, prop.Value.GetIntValue())
				} else if prop.Value.DoubleValue != nil {
					propMap.InsertDouble(prop.Key, prop.Value.GetDoubleValue())
				} else if prop.Value.BoolValue != nil {
					propMap.InsertBool(prop.Key, prop.Value.GetBoolValue())
				}
			}
			propMapVal := pdata.NewAttributeValueMap()
			propMapVal.SetMapVal(propMap)

			attrs.Insert(splunk.SFxEventPropertiesKey, propMapVal)
		}

		lrs.Append(&lr)
	}

	return lrs
}
