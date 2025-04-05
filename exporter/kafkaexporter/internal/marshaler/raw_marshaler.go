// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"

import (
	"encoding/json"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

var (
	errUnsupported               = errors.New("unsupported serialization")
	_              LogsMarshaler = RawLogsMarshaler{}
)

type RawLogsMarshaler struct{}

func (r RawLogsMarshaler) MarshalLogs(logs plog.Logs) ([]Message, error) {
	var messages []Message
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				b, err := r.logBodyAsBytes(lr.Body())
				if err != nil {
					return nil, err
				}
				if len(b) == 0 {
					continue
				}
				messages = append(messages, Message{Value: b})
			}
		}
	}
	return messages, nil
}

func (r RawLogsMarshaler) logBodyAsBytes(value pcommon.Value) ([]byte, error) {
	switch value.Type() {
	case pcommon.ValueTypeStr:
		return r.interfaceAsBytes(value.Str())
	case pcommon.ValueTypeBytes:
		return value.Bytes().AsRaw(), nil
	case pcommon.ValueTypeBool:
		return r.interfaceAsBytes(value.Bool())
	case pcommon.ValueTypeDouble:
		return r.interfaceAsBytes(value.Double())
	case pcommon.ValueTypeInt:
		return r.interfaceAsBytes(value.Int())
	case pcommon.ValueTypeEmpty:
		return []byte{}, nil
	case pcommon.ValueTypeSlice:
		return r.interfaceAsBytes(value.Slice().AsRaw())
	case pcommon.ValueTypeMap:
		return r.interfaceAsBytes(value.Map().AsRaw())
	default:
		return nil, errUnsupported
	}
}

func (r RawLogsMarshaler) interfaceAsBytes(value any) ([]byte, error) {
	if value == nil {
		return []byte{}, nil
	}
	return json.Marshal(value)
}
