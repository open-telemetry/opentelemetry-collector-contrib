// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhec // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/codec/splunkhec"

import (
	"bufio"
	"errors"
	"net/url"
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	// splunk metadata
	index      = "index"
	source     = "source"
	sourcetype = "sourcetype"
	host       = "host"
)

var (
	errCannotConvertValue = errors.New("cannot convert field value to attribute")
)

// splunkHecToLogData transforms splunk events into logs
func splunkHecToLogData(logger *zap.Logger, events []*splunk.Event, config *splunk.HecToOtelAttrs) (plog.Logs, error) {
	ld := plog.NewLogs()
	scopeLogsMap := make(map[[4]string]plog.ScopeLogs)
	for _, event := range events {
		key := [4]string{event.Host, event.Source, event.SourceType, event.Index}
		var sl plog.ScopeLogs
		var found bool
		if sl, found = scopeLogsMap[key]; !found {
			rl := ld.ResourceLogs().AppendEmpty()
			sl = rl.ScopeLogs().AppendEmpty()
			scopeLogsMap[key] = sl
			appendSplunkMetadata(rl, config, event.Host, event.Source, event.SourceType, event.Index)
		}

		// The SourceType field is the most logical "name" of the event.
		logRecord := sl.LogRecords().AppendEmpty()
		if err := convertToValue(logger, event.Event, logRecord.Body()); err != nil {
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
			err := convertToValue(logger, val, logRecord.Attributes().PutEmpty(key))
			if err != nil {
				return ld, err
			}
		}
	}

	return ld, nil
}

// SplunkHecRawToLogData transforms raw splunk event into log
func SplunkHecRawToLogData(sc *bufio.Scanner, query url.Values, resourceCustomizer func(pcommon.Resource), config *splunk.HecToOtelAttrs) (plog.Logs, int) {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	appendSplunkMetadata(rl, config, query.Get(host), query.Get(source), query.Get(sourcetype), query.Get(index))
	if resourceCustomizer != nil {
		resourceCustomizer(rl.Resource())
	}

	sl := rl.ScopeLogs().AppendEmpty()
	for sc.Scan() {
		logRecord := sl.LogRecords().AppendEmpty()
		logLine := sc.Text()
		logRecord.Body().SetStr(logLine)
	}

	return ld, sl.LogRecords().Len()
}

func appendSplunkMetadata(rl plog.ResourceLogs, attrs *splunk.HecToOtelAttrs, host, source, sourceType, index string) {
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

func convertToValue(logger *zap.Logger, src interface{}, dest pcommon.Value) error {
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
	arr := dest.SetEmptySlice()
	for _, elt := range value {
		err := convertToValue(logger, elt, arr.AppendEmpty())
		if err != nil {
			return err
		}
	}
	return nil
}

func convertToAttributeMap(logger *zap.Logger, value map[string]interface{}, dest pcommon.Value) error {
	attrMap := dest.SetEmptyMap()
	keys := make([]string, 0, len(value))
	for k := range value {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := value[k]
		if err := convertToValue(logger, v, attrMap.PutEmpty(k)); err != nil {
			return err
		}
	}
	return nil
}
