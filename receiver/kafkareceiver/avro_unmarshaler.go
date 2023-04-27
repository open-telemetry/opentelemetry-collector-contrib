// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	errNoAvroMapping   = errors.New("no avro field mapping provided")
	errNoAvroSchemaURL = errors.New("no avro url provided")
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

func (a *avroLogsUnmarshaler) Unmarshal(data []byte) (plog.Logs, error) {
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

func (a *avroLogsUnmarshaler) Init(schemaURL string, mapping map[string]string) error {
	a.mapping = mapping

	var schemaDeserializer avroDeserializer
	var err error

	if strings.HasPrefix(schemaURL, avroSchemaPrefixFile) {
		schemaFilePath := strings.TrimPrefix(schemaURL, avroSchemaPrefixFile)
		schemaFilePath = filepath.Clean(schemaFilePath)
		schemaDeserializer, err = newAVROFileSchemaDeserializer(schemaFilePath)
	} else {
		// TODO: schema registry deserializer
		err = fmt.Errorf("unimplemented schema url prefix")
	}

	a.deserializer = schemaDeserializer

	return err
}

type avroDeserializer interface {
	Deserialize([]byte) (map[string]interface{}, error)
}

type avroFileSchemaDeserializer struct {
	codec *goavro.Codec
}

func newAVROFileSchemaDeserializer(path string) (avroDeserializer, error) {
	schema, err := loadAVROSchemaFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read avro schema from file: %w", err)
	}

	codec, err := goavro.NewCodec(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create avro codec: %w", err)
	}

	return &avroFileSchemaDeserializer{
		codec: codec,
	}, nil
}

func loadAVROSchemaFromFile(path string) (string, error) {
	cleanedPath := filepath.Clean(path)
	schema, err := os.ReadFile(cleanedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read schema from file: %w", err)
	}

	return string(schema), nil
}

func (d *avroFileSchemaDeserializer) Deserialize(data []byte) (map[string]interface{}, error) {
	native, _, err := d.codec.NativeFromBinary(data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize avro record: %w", err)
	}

	return native.(map[string]interface{}), nil
}
