// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

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
	var logs []map[string]any

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
				if e.config.Mode == JSONEncodingModeBodyWithInlineAttributes {
					// special handling for inline attributes Mode
					entry := make(map[string]any)

					body := log.Body().AsRaw()
					if body != nil {
						entry["body"] = body
					}

					if len(resourceAttrs) != 0 {
						entry["resourceAttributes"] = resourceAttrs
					}

					logAttribs := log.Attributes().AsRaw()
					if len(logAttribs) != 0 {
						entry["logAttributes"] = logAttribs
					}

					logs = append(logs, entry)
					continue
				}

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
	if !e.config.ArrayMode {
		var buf bytes.Buffer
		for i, log := range logs {
			m, err := json.Marshal(log)
			if err != nil {
				return nil, fmt.Errorf("marshaling error with ndjson log: %w", err)
			}

			buf.Write(m)
			if i < len(logs)-1 {
				// if multiple logs, then consider exporting as ndjson
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
	var err error

	if e.config.ArrayMode {
		// Default mode to handle arrays having backward compatibility
		if err = json.Unmarshal(buf, &jsonLogs); err != nil {
			return p, err
		}
	} else {
		jsonLogs, err = todDecodedJSONDocuments(bytes.NewReader(buf))
		if err != nil {
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

// todDecodedJSONDocuments is a generic helper to derive json records decoded from a reader.
func todDecodedJSONDocuments(reader io.Reader) ([]map[string]any, error) {
	decoder := json.NewDecoder(reader)
	jsonDocuments := make([]map[string]any, 0)

	for {
		var doc map[string]any
		err := decoder.Decode(&doc)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		jsonDocuments = append(jsonDocuments, doc)
	}

	return jsonDocuments, nil
}
