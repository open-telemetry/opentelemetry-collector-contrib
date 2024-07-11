// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package avrologencodingextension

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewAvroLogsMarshaler(t *testing.T) {
	schema, jsonMap := createMapTestData(t)

	avroEncoder, err := newAVROStaticSchemaSerDe(schema)
	if err != nil {
		t.Errorf("Did not expect an error, got %q", err.Error())
	}

	avroData, err := avroEncoder.Serialize(jsonMap, 0)
	if err != nil {
		t.Fatalf("Did not expect an error, got %q", err.Error())
	}

	logMap, err := avroEncoder.Deserialize(avroData)
	if err != nil {
		t.Fatalf("Did not expect an error, got %q", err.Error())
	}

	assert.Equal(t, int64(1697187201488000000), logMap["timestamp"].(time.Time).UnixNano())
	assert.Equal(t, "host1", logMap["hostname"])
	assert.Equal(t, int64(12), logMap["nestedRecord"].(map[string]any)["field1"])

	props := logMap["properties"].([]any)
	propsStr := make([]string, len(props))
	for i, prop := range props {
		propsStr[i] = prop.(string)
	}

	assert.Equal(t, []string{"prop1", "prop2"}, propsStr)
}

func TestNewAvroLogsMarshalerInvalidData(t *testing.T) {
	schema, _ := createMapTestData(t)

	avroEncoder, err := newAVROStaticSchemaSerDe(schema)
	if err != nil {
		t.Errorf("Did not expect an error, got %q", err.Error())
	}

	_, err = avroEncoder.Serialize(map[string]any{"invalid": "data"}, 0)
	assert.Error(t, err)
}

func TestSchemaID(t *testing.T) {
	schema, jsonMap := createMapTestData(t)

	avroEncoder, err := newAVROStaticSchemaSerDe(schema)
	if err != nil {
		t.Errorf("Did not expect an error, got %q", err.Error())
	}

	schemaID := uint32(4294967295) // This number is 32 bits of all 1s

	avroData, err := avroEncoder.Serialize(jsonMap, schemaID)
	if err != nil {
		t.Fatalf("Did not expect an error, got %q", err.Error())
	}

	binarySchemaID := []byte{0, 0, 0, 0}
	binary.BigEndian.PutUint32(binarySchemaID, schemaID)

	assert.Equal(t, avroData[0], uint8(0x0))
	assert.Equal(t, avroData[1:5], binarySchemaID)

	_, err = avroEncoder.Deserialize(avroData[5:])
	if err != nil {
		t.Fatalf("Did not expect an error, got %q", err.Error())
	}
}

func TestNewAvroLogsSerDeDeserialize(t *testing.T) {
	schema, data := createAVROTestData(t)

	deserializer, err := newAVROStaticSchemaSerDe(schema)
	if err != nil {
		t.Errorf("Did not expect an error, got %q", err.Error())
	}

	logMap, err := deserializer.Deserialize(data)
	if err != nil {
		t.Fatalf("Did not expect an error, got %q", err.Error())
	}

	assert.Equal(t, int64(1697187201488000000), logMap["timestamp"].(time.Time).UnixNano())
	assert.Equal(t, "host1", logMap["hostname"])
	assert.Equal(t, int64(12), logMap["nestedRecord"].(map[string]any)["field1"])

	props := logMap["properties"].([]any)
	propsStr := make([]string, len(props))
	for i, prop := range props {
		propsStr[i] = prop.(string)
	}

	assert.Equal(t, []string{"prop1", "prop2"}, propsStr)
}

func TestNewAvroLogsSerDeInvalidSchema(t *testing.T) {
	_, err := newAVROStaticSchemaSerDe("invalid schema")
	assert.Error(t, err)
}
