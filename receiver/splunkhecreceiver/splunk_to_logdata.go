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

package splunkhecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"

import (
	"errors"
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

var (
	errCannotConvertValue = errors.New("cannot convert field value to attribute")
)

// splunkHecToLogData transforms splunk events into logs
func splunkHecToLogData(logger *zap.Logger, events []*splunk.Event, resourceCustomizer func(pcommon.Resource), config *Config) (plog.Logs, error) {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	for _, event := range events {
		// The SourceType field is the most logical "name" of the event.
		logRecord := sl.LogRecords().AppendEmpty()
		if err := convertToValue(logger, event.Event, logRecord.Body()); err != nil {
			return ld, err
		}

		// Splunk timestamps are in seconds so convert to nanos by multiplying
		// by 1 billion.
		if event.Time != nil {
			logRecord.SetTimestamp(pcommon.Timestamp(*event.Time * 1e9))
		}

		// Set event fields first, so the specialized attributes overwrite them if needed.
		keys := make([]string, 0, len(event.Fields))
		for k := range event.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := event.Fields[key]
			err := convertToValue(logger, val, logRecord.Attributes().UpsertEmpty(key))
			if err != nil {
				return ld, err
			}
		}

		if event.Host != "" {
			logRecord.Attributes().UpsertString(config.HecToOtelAttrs.Host, event.Host)
		}
		if event.Source != "" {
			logRecord.Attributes().UpsertString(config.HecToOtelAttrs.Source, event.Source)
		}
		if event.SourceType != "" {
			logRecord.Attributes().UpsertString(config.HecToOtelAttrs.SourceType, event.SourceType)
		}
		if event.Index != "" {
			logRecord.Attributes().UpsertString(config.HecToOtelAttrs.Index, event.Index)
		}
		if resourceCustomizer != nil {
			resourceCustomizer(rl.Resource())
		}
	}

	return ld, nil
}

func convertToValue(logger *zap.Logger, src interface{}, dest pcommon.Value) error {
	switch value := src.(type) {
	case nil:
	case string:
		dest.SetStringVal(value)
	case int64:
		dest.SetIntVal(value)
	case float64:
		dest.SetDoubleVal(value)
	case bool:
		dest.SetBoolVal(value)
	case map[string]interface{}:
		return convertToAttributeMap(logger, value, dest)
	case []interface{}:
		return convertToSliceVal(logger, value, dest)
	default:
		logger.Debug("Unsupported value conversion", zap.Any("value", src))
		return errCannotConvertValue

	}
	return nil
}

func convertToSliceVal(logger *zap.Logger, value []interface{}, dest pcommon.Value) error {
	arr := dest.SetEmptySliceVal()
	for _, elt := range value {
		err := convertToValue(logger, elt, arr.AppendEmpty())
		if err != nil {
			return err
		}
	}
	return nil
}

func convertToAttributeMap(logger *zap.Logger, value map[string]interface{}, dest pcommon.Value) error {
	attrMap := dest.SetEmptyMapVal()
	keys := make([]string, 0, len(value))
	for k := range value {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := value[k]
		if err := convertToValue(logger, v, attrMap.UpsertEmpty(k)); err != nil {
			return err
		}
	}
	return nil
}
