// Copyright The OpenTelemetry Authors
// Copyright 2026-present Datadog, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package di converts versioned DI LogRecords to existing debugger uploads.
package di // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/di"

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// Validate the transport envelope and JSON shape here. Full per-language
// product schema validation is deferred; preserve native content verbatim.
func encodeDIEvent(record plog.LogRecord) ([]byte, string, error) {
	version, ok := record.Attributes().Get("datadog.di.schema_version")
	if !ok || version.Type() != pcommon.ValueTypeInt || version.Int() != 1 {
		return nil, "", errors.New("unsupported DI event schema")
	}
	payload, ok := record.Attributes().Get("datadog.di.payload")
	if ok && payload.Type() == pcommon.ValueTypeStr && len(payload.Str()) > 5<<20 {
		return nil, "", errors.New("DI payload exceeds the experimental 5 MiB limit")
	}
	if !ok || payload.Type() != pcommon.ValueTypeStr || !json.Valid([]byte(payload.Str())) || !strings.HasPrefix(strings.TrimSpace(payload.Str()), "{") {
		return nil, "", errors.New("DI event requires a JSON object payload")
	}
	switch record.EventName() {
	case "dd.debugger.snapshot", "dd.debugger.log":
		return []byte("[" + payload.Str() + "]"), "application/json", nil
	case "dd.debugger.diagnostic":
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", `form-data; name="event"; filename="event.json"`)
		header.Set("Content-Type", "application/json")
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err := io.WriteString(part, "["+payload.Str()+"]"); err != nil {
			return nil, "", err
		}
		if err := writer.Close(); err != nil {
			return nil, "", err
		}
		return body.Bytes(), writer.FormDataContentType(), nil
	case "dd.debugger.symbols":
		return encodeSymbols([]byte(payload.Str()))
	default:
		return nil, "", errors.New("unsupported DI event name")
	}
}

func encodeSymbols(payload []byte) ([]byte, string, error) {
	var envelope struct {
		Event   map[string]json.RawMessage `json:"event"`
		Symbols json.RawMessage            `json:"symbols"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, "", err
	}
	if len(envelope.Event) == 0 || len(envelope.Symbols) == 0 || !strings.HasPrefix(strings.TrimSpace(string(envelope.Symbols)), "{") {
		return nil, "", errors.New("symbol event requires metadata and a symbols object")
	}
	var compressed bytes.Buffer
	zip := gzip.NewWriter(&compressed)
	if _, err := zip.Write(envelope.Symbols); err != nil {
		return nil, "", err
	}
	if err := zip.Close(); err != nil {
		return nil, "", err
	}
	envelope.Event["attachmentSize"] = json.RawMessage(strconv.Itoa(compressed.Len()))
	event, err := json.Marshal(envelope.Event)
	if err != nil {
		return nil, "", err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, part := range []struct {
		name, file, contentType string
		data                    []byte
	}{
		{"event", "event.json", "application/json", event},
		{"file", "symbols.json.gz", "application/gzip", compressed.Bytes()},
	} {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, part.name, part.file))
		header.Set("Content-Type", part.contentType)
		w, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err := w.Write(part.data); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}
