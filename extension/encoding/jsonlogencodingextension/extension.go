// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"

import (
	"bufio"
	"bytes"
	"context"
	"fmt"

	"github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var (
	_ encoding.LogsMarshalerExtension   = (*jsonLogExtension)(nil)
	_ encoding.LogsUnmarshalerExtension = (*jsonLogExtension)(nil)
)

type jsonLogExtension struct {
	config *Config
}

func (e *jsonLogExtension) MarshalLogs(ld plog.Logs) ([]byte, error) {
	if e.config.Mode == JSONEncodingModeBodyWithInlineAttributes {
		return e.logProcessor(ld)
	}
	logs := make([]map[string]any, 0, ld.LogRecordCount())

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			logSlice := sl.LogRecords()
			for k := 0; k < logSlice.Len(); k++ {
				log := logSlice.At(k)
				switch log.Body().Type() {
				case pcommon.ValueTypeMap:
					logs = append(logs, log.Body().Map().AsRaw())
				default:
					return nil, fmt.Errorf("marshal: expected 'Map' found '%v'", log.Body().Type())
				}
			}
		}
	}

	// check for processing mode so we can return the best format
	switch e.config.ProcessingMode {
	case SingleMode:
		if len(logs) == 1 {
			return json.Marshal(logs[0])
		}
	case NDJsonMode:
		var buf bytes.Buffer

		for i, log := range logs {
			m, err := json.Marshal(log)
			if err != nil {
				return nil, fmt.Errorf("marshaling error with ndjson log: %w", err)
			}

			buf.Write(m)
			if i < len(logs)-1 {
				buf.WriteByte('\n')
			}
		}

		return buf.Bytes(), nil
	}

	// default mode
	return json.Marshal(logs)
}

func (e *jsonLogExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	p := plog.NewLogs()

	// get json logs from the buffer
	var jsonLogs []map[string]any
	switch e.config.ProcessingMode {
	case ArrayMode:
		if err := json.Unmarshal(buf, &jsonLogs); err != nil {
			return p, err
		}
	case SingleMode:
		var doc map[string]any
		if err := json.Unmarshal(buf, &doc); err != nil {
			return p, err
		}
		jsonLogs = append(jsonLogs, doc)
	case NDJsonMode:
		sc := bufio.NewScanner(bytes.NewReader(buf))
		for sc.Scan() {
			var line map[string]any
			err := json.Unmarshal(sc.Bytes(), &line)
			if err != nil {
				return p, err
			}

			jsonLogs = append(jsonLogs, line)
		}

		if err := sc.Err(); err != nil {
			// consider this as an error
			return p, err
		}
	}

	sl := p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
	for _, r := range jsonLogs {
		if err := sl.LogRecords().AppendEmpty().Body().SetEmptyMap().FromRaw(r); err != nil {
			return p, err
		}
	}

	return p, nil
}

func (e *jsonLogExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *jsonLogExtension) Shutdown(_ context.Context) error {
	return nil
}

func (e *jsonLogExtension) logProcessor(ld plog.Logs) ([]byte, error) {
	logs := make([]logBody, 0, ld.LogRecordCount())

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resourceAttrs := rl.Resource().Attributes().AsRaw()

		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			logSlice := sl.LogRecords()
			for k := 0; k < logSlice.Len(); k++ {
				log := logSlice.At(k)
				logEvent := logBody{
					Body:               log.Body().AsRaw(),
					ResourceAttributes: resourceAttrs,
					LogAttributes:      log.Attributes().AsRaw(),
				}
				logs = append(logs, logEvent)
			}
		}
	}

	return json.Marshal(logs)
}

type logBody struct {
	Body               any            `json:"body,omitempty"`
	LogAttributes      map[string]any `json:"logAttributes,omitempty"`
	ResourceAttributes map[string]any `json:"resourceAttributes,omitempty"`
}
