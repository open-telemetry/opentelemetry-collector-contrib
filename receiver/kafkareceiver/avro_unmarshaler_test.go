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

package kafkareceiver

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/linkedin/goavro/v2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestNewAvroLogsUnmarshaler(t *testing.T) {
	schema, err := loadAVROSchemaFromFile("testdata/avro/schema1.avro")
	if err != nil {
		t.Fatalf("Failed to read avro schema file: %q", err.Error())
	}
	codec, err := goavro.NewCodec(schema)
	if err != nil {
		t.Fatalf("Failed to create avro code from schema: %q", err.Error())
	}

	unmarshaler := newAVROLogsUnmarshaler()
	if err = unmarshaler.Init(
		"file:testdata/avro/schema1.avro",
		map[string]string{
			"timestamp":    "timestamp",
			"properties":   "resource.attributes.properties",
			"hostname":     "resource.attributes.hostname",
			"count":        "attributes.count",
			"message":      "body",
			"nestedRecord": "attributes.nestedRecord",
			"levelEnum":    "severityText",
			"severity":     "severityNumber",
			"doesnotexist": "attributes.doesnotexist",
		},
	); err != nil {
		t.Errorf("Did not expect an error, got %q", err.Error())
	}

	timeNow := time.Now()
	binary := encodeAVROLogTestData(codec, `{
		"timestamp": `+strconv.Itoa(int(timeNow.UnixMilli()))+`,
		"hostname": "host1",
		"message": "log message",
		"count": 5,
		"nestedRecord": {
			"field1": 12
		},
		"properties": ["prop1", "prop2"],
		"severity": 1
	}`)

	logs, err := unmarshaler.Unmarshal(binary)
	if err != nil {
		t.Fatalf("Did not expect an error, got %q", err.Error())
	}

	resourceAttributes := logs.ResourceLogs().At(0).Resource().Attributes()

	hostnameAttribute, _ := resourceAttributes.Get("hostname")
	assert.Equal(t, "host1", hostnameAttribute.Str())

	logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, "log message", logRecord.Body().AsString())
	assert.Equal(t, timeNow.UnixMilli(), logRecord.Timestamp().AsTime().UnixMilli())
	assert.Equal(t, "INFO", logRecord.SeverityText())
	assert.Equal(t, plog.SeverityNumber(1), logRecord.SeverityNumber())

	assert.Equal(t, 2, logRecord.Attributes().Len())
	_, ok := logRecord.Attributes().Get("doesnotexist")
	assert.False(t, ok)

	nestedRecordAttribute, _ := logRecord.Attributes().Get("nestedRecord")
	field1, _ := nestedRecordAttribute.Map().Get("field1")
	assert.Equal(t, int64(12), field1.Int())
	field2, _ := nestedRecordAttribute.Map().Get("field2")
	assert.Equal(t, "val2", field2.Str())

	propertiesAttribute, _ := resourceAttributes.Get("properties")
	for idx, expectedValue := range []string{"prop1", "prop2"} {
		assert.Equal(t, expectedValue, propertiesAttribute.Slice().At(idx).Str())
	}
}

func encodeAVROLogTestData(codec *goavro.Codec, data string) []byte {
	textual := []byte(data)
	native, _, err := codec.NativeFromTextual(textual)
	if err != nil {
		fmt.Println(err)
	}

	binary, err := codec.BinaryFromNative(nil, native)
	if err != nil {
		fmt.Println(err)
	}

	return binary
}
