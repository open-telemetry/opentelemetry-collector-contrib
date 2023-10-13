// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/linkedin/goavro/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	avroEncoding         = "avro"
	avroSchemaPrefixFile = "file:"
)

var (
	errFailedToReadAvroSchema = errors.New("failed to read avro schema")
)

type avroLogsUnmarshaler struct {
	deserializer avroDeserializer
}

func newAVROLogsUnmarshaler() *avroLogsUnmarshaler {
	return &avroLogsUnmarshaler{}
}

func (a *avroLogsUnmarshaler) Encoding() string {
	return avroEncoding
}

func (a *avroLogsUnmarshaler) WithSchema(schemaReader io.Reader) (*avroLogsUnmarshaler, error) {
	schema, err := io.ReadAll(schemaReader)
	if err != nil {
		return nil, errFailedToReadAvroSchema
	}

	deserializer, err := newAVROStaticSchemaDeserializer(string(schema))
	a.deserializer = deserializer

	return a, err
}

func (a *avroLogsUnmarshaler) Unmarshal(data []byte) (plog.Logs, error) {
	p := plog.NewLogs()

	if a.deserializer == nil {
		return p, errors.New("avro deserializer not set")
	}

	avroLog, err := a.deserializer.Deserialize(data)
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

type avroDeserializer interface {
	Deserialize([]byte) (map[string]any, error)
}

type avroStaticSchemaDeserializer struct {
	codec *goavro.Codec
}

func newAVROStaticSchemaDeserializer(schema string) (avroDeserializer, error) {
	codec, err := goavro.NewCodec(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create avro codec: %w", err)
	}

	return &avroStaticSchemaDeserializer{
		codec: codec,
	}, nil
}

func (d *avroStaticSchemaDeserializer) Deserialize(data []byte) (map[string]any, error) {
	native, _, err := d.codec.NativeFromBinary(data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize avro record: %w", err)
	}

	return native.(map[string]interface{}), nil
}
