// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"

import (
	"context"
	"errors"
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type jsonExtension struct {
}

func (e *jsonExtension) MarshalLogs(ld plog.Logs) ([]byte, error) {
	logRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()
	var raw map[string]any
	switch logRecord.Type() {
	case pcommon.ValueTypeMap:
		raw = logRecord.Map().AsRaw()
	default:
		return nil, errors.New(fmt.Sprintf("Marshal: Expected 'Map' found '%v'", logRecord.Type().String()))
	}
	if buf, err := jsoniter.Marshal(raw); err != nil {
		return nil, err
	} else {
		return buf, nil
	}
}

func (e *jsonExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	p := plog.NewLogs()

	// get json logs from the buffer
	jsonVal := map[string]interface{}{}
	if err := jsoniter.Unmarshal(buf, &jsonVal); err != nil {
		return p, err
	}

	// create a new log record
	logRecords := p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecords.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Set the unmarshaled jsonVal as the body of the log record
	if err := logRecords.Body().SetEmptyMap().FromRaw(jsonVal); err != nil {
		return p, err
	}
	return p, nil
}

func (e *jsonExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *jsonExtension) Shutdown(_ context.Context) error {
	return nil
}
