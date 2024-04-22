// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"

import (
	"errors"
	"sort"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	// splunk metadata
	index      = "index"
	source     = "source"
	sourcetype = "sourcetype"
	host       = "host"
	queryTime  = "time"
)

var (
	errCannotConvertValue = errors.New("cannot convert field value to attribute")
)

// splunkHecToLogData transforms splunk events into logs
func splunkHecToLogData(event splunk.Event, config *Config) (plog.Logs, error) {
	ld := plog.NewLogs()
	scopeLogsMap := make(map[[4]string]plog.ScopeLogs)
	key := [4]string{event.Host, event.Source, event.SourceType, event.Index}
	var sl plog.ScopeLogs
	var found bool
	if sl, found = scopeLogsMap[key]; !found {
		rl := ld.ResourceLogs().AppendEmpty()
		sl = rl.ScopeLogs().AppendEmpty()
		scopeLogsMap[key] = sl
		appendSplunkMetadata(rl, config.HecToOtelAttrs, event.Host, event.Source, event.SourceType, event.Index)
	}

	// The SourceType field is the most logical "name" of the event.
	logRecord := sl.LogRecords().AppendEmpty()
	if err := convertToValue(event.Event, logRecord.Body()); err != nil {
		return ld, err
	}

	// Splunk timestamps are in seconds so convert to nanos by multiplying
	// by 1 billion.
	logRecord.SetTimestamp(pcommon.Timestamp(event.Time * 1e9))

	// Set event fields first, so the specialized attributes overwrite them if needed.
	keys := make([]string, 0, len(event.Fields))
	for k := range event.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		val := event.Fields[key]
		err := convertToValue(val, logRecord.Attributes().PutEmpty(key))
		if err != nil {
			return ld, err
		}
	}

	return ld, nil
}

func appendSplunkMetadata(rl plog.ResourceLogs, attrs splunk.HecToOtelAttrs, host, source, sourceType, index string) {
	if host != "" {
		rl.Resource().Attributes().PutStr(attrs.Host, host)
	}
	if source != "" {
		rl.Resource().Attributes().PutStr(attrs.Source, source)
	}
	if sourceType != "" {
		rl.Resource().Attributes().PutStr(attrs.SourceType, sourceType)
	}
	if index != "" {
		rl.Resource().Attributes().PutStr(attrs.Index, index)
	}
}

func convertToValue(src any, dest pcommon.Value) error {
	switch value := src.(type) {
	case nil:
	case string:
		dest.SetStr(value)
	case int64:
		dest.SetInt(value)
	case float64:
		dest.SetDouble(value)
	case bool:
		dest.SetBool(value)
	case map[string]any:
		return convertToAttributeMap(value, dest)
	case []any:
		return convertToSliceVal(value, dest)
	default:
		return errCannotConvertValue

	}
	return nil
}

func convertToSliceVal(value []any, dest pcommon.Value) error {
	arr := dest.SetEmptySlice()
	for _, elt := range value {
		err := convertToValue(elt, arr.AppendEmpty())
		if err != nil {
			return err
		}
	}
	return nil
}

func convertToAttributeMap(value map[string]any, dest pcommon.Value) error {
	attrMap := dest.SetEmptyMap()
	keys := make([]string, 0, len(value))
	for k := range value {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := value[k]
		if err := convertToValue(v, attrMap.PutEmpty(k)); err != nil {
			return err
		}
	}
	return nil
}
