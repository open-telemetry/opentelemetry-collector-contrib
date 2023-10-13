// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/linkedin/goavro/v2"
	"github.com/stretchr/testify/assert"
)

func TestNewAvroLogsUnmarshaler(t *testing.T) {
	schema, err := loadAVROSchemaFromFile("testdata/avro/schema1.avro")
	if err != nil {
		t.Fatalf("Failed to read avro schema file: %q", err.Error())
	}
	codec, err := goavro.NewCodec(string(schema))
	if err != nil {
		t.Fatalf("Failed to create avro code from schema: %q", err.Error())
	}

	unmarshaler, err := newAVROLogsUnmarshaler().WithSchema(bytes.NewReader(schema))
	if err != nil {
		t.Errorf("Did not expect an error, got %q", err.Error())
	}

	binary := encodeAVROLogTestData(codec, `{
		"timestamp": 1697187201488,
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

	logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, "{\"count\":5,\"hostname\":\"host1\",\"level\":\"warn\",\"levelEnum\":\"INFO\",\"mapField\":{},\"message\":\"log message\",\"nestedRecord\":{\"field1\":12,\"field2\":\"val2\"},\"properties\":[\"prop1\",\"prop2\"],\"severity\":1,\"timestamp\":1697187201488000000}", logRecord.Body().AsString())
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

func loadAVROSchemaFromFile(path string) ([]byte, error) {
	cleanedPath := filepath.Clean(path)
	schema, err := os.ReadFile(cleanedPath)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to read schema from file: %w", err)
	}

	return schema, nil
}
