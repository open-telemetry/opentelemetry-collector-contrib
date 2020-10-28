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

package translation

import (
	"fmt"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func LogSliceToSignalFxV2(
	logger *zap.Logger,
	logs pdata.LogSlice,
) ([]*sfxpb.Event, int) {
	events := make([]*sfxpb.Event, 0, logs.Len())
	numDroppedLogRecords := 0

	for i := 0; i < logs.Len(); i++ {
		lr := logs.At(i)
		event, ok := convertLogRecord(lr, logger)
		if !ok {
			numDroppedLogRecords++
			continue
		}
		events = append(events, event)
	}

	return events, numDroppedLogRecords
}

func convertLogRecord(lr pdata.LogRecord, logger *zap.Logger) (*sfxpb.Event, bool) {
	attrs := lr.Attributes()

	categoryVal, ok := attrs.Get(splunk.SFxEventCategoryKey)
	if !ok {
		return nil, false
	}

	var event sfxpb.Event

	if categoryVal.Type() == pdata.AttributeValueINT {
		asCat := sfxpb.EventCategory(categoryVal.IntVal())
		event.Category = &asCat
		attrs.Delete(splunk.SFxEventCategoryKey)
	}

	if mapVal, ok := attrs.Get(splunk.SFxEventPropertiesKey); ok && mapVal.Type() == pdata.AttributeValueMAP {
		mapVal.MapVal().ForEach(func(k string, v pdata.AttributeValue) {
			val, err := attributeValToPropertyVal(v)
			if err != nil {
				logger.Debug("Failed to convert log record property value to SignalFx property value", zap.Error(err), zap.String("key", k))
				return
			}

			event.Properties = append(event.Properties, &sfxpb.Property{
				Key:   k,
				Value: val,
			})
		})
	}
	attrs.Delete(splunk.SFxEventPropertiesKey)

	attrs.ForEach(func(k string, v pdata.AttributeValue) {
		if v.Type() != pdata.AttributeValueSTRING {
			logger.Debug("Failed to convert log record attribute value to SignalFx property value, key is not a string", zap.String("key", k))
			return
		}

		event.Dimensions = append(event.Dimensions, &sfxpb.Dimension{
			Key:   k,
			Value: v.StringVal(),
		})
	})

	event.EventType = lr.Name()
	// Convert nanoseconds to nearest milliseconds, which is the unit of
	// SignalFx event timestamps.
	event.Timestamp = int64(lr.Timestamp()) / 1e6

	return &event, true
}

func attributeValToPropertyVal(v pdata.AttributeValue) (*sfxpb.PropertyValue, error) {
	var val sfxpb.PropertyValue
	switch v.Type() {
	case pdata.AttributeValueINT:
		asInt := v.IntVal()
		val.IntValue = &asInt
	case pdata.AttributeValueBOOL:
		asBool := v.BoolVal()
		val.BoolValue = &asBool
	case pdata.AttributeValueDOUBLE:
		asDouble := v.DoubleVal()
		val.DoubleValue = &asDouble
	case pdata.AttributeValueSTRING:
		asString := v.StringVal()
		val.StrValue = &asString
	default:
		return nil, fmt.Errorf("attribute value type %q not supported in SignalFx events", v.Type().String())
	}

	return &val, nil
}
