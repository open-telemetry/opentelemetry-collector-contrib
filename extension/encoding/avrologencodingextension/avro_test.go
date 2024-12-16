// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package avrologencodingextension

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAvroLogsUnmarshaler(t *testing.T) {
	schema, data := createAVROTestData(t)

	deserializer, err := newAVROStaticSchemaDeserializer(schema)
	require.NoError(t, err, "Did not expect an error")

	logMap, err := deserializer.Deserialize(data)
	require.NoError(t, err, "Did not expect an error")

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

func TestNewAvroLogsUnmarshalerInvalidSchema(t *testing.T) {
	_, err := newAVROStaticSchemaDeserializer("invalid schema")
	assert.Error(t, err)
}
