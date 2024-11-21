// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package avrologencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/avrologencodingextension"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var _ encoding.LogsUnmarshalerExtension = (*avroLogExtension)(nil)

type avroLogExtension struct {
	deserializer avroDeserializer
}

func newExtension(config *Config) (*avroLogExtension, error) {
	deserializer, err := newAVROStaticSchemaDeserializer(config.Schema)
	if err != nil {
		return nil, err
	}

	return &avroLogExtension{deserializer: deserializer}, nil
}

func (e *avroLogExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	p := plog.NewLogs()

	avroLog, err := e.deserializer.Deserialize(buf)
	if err != nil {
		return p, fmt.Errorf("failed to deserialize avro log: %w", err)
	}

	logRecords := p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecords.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// removes time.Time values as FromRaw does not support it
	replaceLogicalTypes(avroLog)

	// Set the unmarshaled avro as the body of the log record
	if err := logRecords.Body().SetEmptyMap().FromRaw(avroLog); err != nil {
		return p, err
	}

	return p, nil
}

func replaceLogicalTypes(m map[string]any) {
	for k, v := range m {
		m[k] = transformValue(v)
	}
}

func transformValue(value any) any {
	if timeValue, ok := value.(time.Time); ok {
		return timeValue.UnixNano()
	}

	if mapValue, ok := value.(map[string]any); ok {
		replaceLogicalTypes(mapValue)
		return mapValue
	}

	if arrayValue, ok := value.([]any); ok {
		for i, v := range arrayValue {
			arrayValue[i] = transformValue(v)
		}
		return arrayValue
	}

	return value
}

func (e *avroLogExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *avroLogExtension) Shutdown(_ context.Context) error {
	return nil
}
