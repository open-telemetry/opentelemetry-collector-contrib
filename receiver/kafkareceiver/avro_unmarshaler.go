// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/linkedin/goavro/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	avroEncoding              = "avro"
	avroSchemaPrefixFile      = "file:"
	logRecordAttributesPrefix = "attributes."
	resourceAttributesPrefix  = "resource.attributes."
	bodyField                 = "body"
	timestampField            = "timestamp"
	severityTextField         = "severityText"
	severityNumberField       = "severityNumber"
)

var (
	errFailedToReadAvroSchema = errors.New("failed to read avro schema")
	errNoAvroMapping          = errors.New("no avro field mapping provided")
)

type avroLogsUnmarshaler struct {
	deserializer avroDeserializer
	mapping      map[string]string
}

func newAVROLogsUnmarshaler() *avroLogsUnmarshaler {
	return &avroLogsUnmarshaler{}
}

func (a *avroLogsUnmarshaler) Encoding() string {
	return avroEncoding
}

func (a *avroLogsUnmarshaler) WithSchema(mapping map[string]string, schemaReader io.Reader) (*avroLogsUnmarshaler, error) {
	if mapping == nil {
		return nil, errNoAvroMapping
	}

	a.mapping = mapping

	schema, err := io.ReadAll(schemaReader)
	if err != nil {
		return nil, errFailedToReadAvroSchema
	}

	deserializer, err := newAVROStaticSchemaDeserializer(string(schema))
	a.deserializer = deserializer

	return a, err
}

func (a *avroLogsUnmarshaler) Unmarshal(data []byte) (plog.Logs, error) {
	if a.deserializer == nil {
		return plog.Logs{}, errors.New("avro deserializer not set")
	}

	avroLog, err := a.deserializer.Deserialize(data)
	if err != nil {
		return plog.NewLogs(), fmt.Errorf("failed to deserialize avro log: %w", err)
	}

	return a.mapLog(avroLog), nil
}

func (a *avroLogsUnmarshaler) mapLog(avroLog map[string]interface{}) plog.Logs {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceAttributes := resourceLogs.Resource().Attributes()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	logRecordAttributes := logRecord.Attributes()

	for sourceField, target := range a.mapping {
		switch {
		case isResourceAttribute(target):
			targetField := strings.TrimPrefix(target, resourceAttributesPrefix)
			mapAttribute(avroLog, sourceField, &resourceAttributes, targetField)
		case isLogRecordAttribute(target):
			targetField := strings.TrimPrefix(target, logRecordAttributesPrefix)
			mapAttribute(avroLog, sourceField, &logRecordAttributes, targetField)
		case target == bodyField:
			logRecord.Body().SetStr(avroLog[sourceField].(string))
		case target == timestampField:
			timestamp := avroLog[timestampField].(time.Time)
			logRecord.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		case target == severityTextField:
			logRecord.SetSeverityText(avroLog[sourceField].(string))
		case target == severityNumberField:
			severityNumber, ok := avroLog[sourceField].(int32)
			if !ok {
				severityNumber = 0
			}
			logRecord.SetSeverityNumber(plog.SeverityNumber(severityNumber))
		}
	}

	return logs
}

func isResourceAttribute(targetField string) bool {
	return strings.HasPrefix(targetField, resourceAttributesPrefix)
}

func isLogRecordAttribute(targetField string) bool {
	return strings.HasPrefix(targetField, logRecordAttributesPrefix)
}

func mapAttribute(avroLog map[string]interface{}, sourceField string, attributes *pcommon.Map, targetField string) {
	sourceValue := avroLog[sourceField]
	if sourceValue != nil {
		setAttribute(attributes, targetField, sourceValue)
	}
}

func setAttribute(attributes *pcommon.Map, key string, value interface{}) {
	attribute := attributes.PutEmpty(key)
	_ = attribute.FromRaw(value)
}

type avroDeserializer interface {
	Deserialize([]byte) (map[string]interface{}, error)
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

func (d *avroStaticSchemaDeserializer) Deserialize(data []byte) (map[string]interface{}, error) {
	native, _, err := d.codec.NativeFromBinary(data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize avro record: %w", err)
	}

	return native.(map[string]interface{}), nil
}
