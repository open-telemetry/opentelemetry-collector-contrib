// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

import (
	"context"
	stdjson "encoding/json"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
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

func (ex *ext) Shutdown(_ context.Context) error {
	return nil
}

func (ex *ext) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	out := plog.NewLogs()

	resource, lr, err := ex.translateLogEntry(buf)
	if err != nil {
		return out, err
	}

	logs := out.ResourceLogs()
	rls := logs.AppendEmpty()
	resource.CopyTo(rls.Resource())

	ills := rls.ScopeLogs().AppendEmpty()
	lr.CopyTo(ills.LogRecords().AppendEmpty())
	return out, nil
}

// TranslateLogEntry translates a JSON-encoded LogEntry message into a pair of
// pcommon.Resource and plog.LogRecord, trying to keep as close as possible to
// the semantic conventions.
//
// For maximum fidelity, the decoding is done according to the protobuf message
// schema; this ensures that a numeric value in the input is correctly
// translated to either an integer or a double in the output. It falls back to
// plain JSON decoding if payload type is not available in the proto registry.
func (ex *ext) translateLogEntry(data []byte) (pcommon.Resource, plog.LogRecord, error) {
	lr := plog.NewLogRecord()
	res := pcommon.NewResource()

	var src map[string]stdjson.RawMessage
	err := json.Unmarshal(data, &src)
	if err != nil {
		return res, lr, err
	}

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
	return res, lr, err
}
