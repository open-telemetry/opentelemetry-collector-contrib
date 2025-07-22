// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

import (
	"context"
	stdjson "encoding/json"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var _ encoding.LogsUnmarshalerExtension = (*ext)(nil)

type ext struct {
	Config     Config
	extractFns map[string]extractFn
}

func newExtension(cfg *Config) *ext {
	return &ext{Config: *cfg}
}

func (ex *ext) Start(_ context.Context, _ component.Host) error {
	ex.extractFns = map[string]extractFn{
		"protoPayload":     handleProtoPayload,
		"receiveTimestamp": handleReceiveTimestamp,
		"timestamp":        handleTimestamp,
		"insertId":         handleInsertID,
		"logName":          handleLogName,
		"textPayload":      handleTextPayload,
		"jsonPayload":      handleJSONPayload,
		"severity":         handleSeverity,
		"trace":            handleTrace,
		"spanId":           handleSpanID,
		"traceSampled":     handleTraceSampled,
		"labels":           handleLabels,
		"httpRequest":      handleHTTPRequest,
		"resource":         handleResource,
	}
	return nil
}

func (*ext) Shutdown(context.Context) error {
	return nil
}

func (ex *ext) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	logs, err := ex.translateLogEntry(buf)
	if err != nil {
		return plog.Logs{}, err
	}

	return logs, nil
}

// TranslateLogEntry translates a JSON-encoded LogEntry message into a pair of
// pcommon.Resource and plog.LogRecord, trying to keep as close as possible to
// the semantic conventions.
//
// For maximum fidelity, the decoding is done according to the protobuf message
// schema; this ensures that a numeric value in the input is correctly
// translated to either an integer or a double in the output. It falls back to
// plain JSON decoding if payload type is not available in the proto registry.
func (ex *ext) translateLogEntry(data []byte) (plog.Logs, error) {
	var src map[string]stdjson.RawMessage
	err := json.Unmarshal(data, &src)
	if err != nil {
		return plog.Logs{}, err
	}

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	res := rl.Resource()
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	resAttrs := res.Attributes()
	attrs := lr.Attributes()

	for key, value := range src {
		fn := ex.extractFns[key]
		if fn != nil {
			kverr := fn(resAttrs, lr, attrs, key, value, ex.Config)
			if kverr == nil {
				delete(src, key)
			}
		}
	}

	// All other fields -> Attributes["gcp.*"]
	err = translateInto(attrs, getLogEntryDescriptor(), src, preserveDst, prefixKeys("gcp."), snakeifyKeys)
	return logs, err
}
