// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/elastic/go-structform"
	structformjson "github.com/elastic/go-structform/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestWriteAttributes_Deduplication(t *testing.T) {
	// Test that writeAttributes deduplicates keys by keeping the first occurrence.
	// Parse OTLP JSON with duplicate attribute keys to create a pcommon.Map with duplicates.
	otlpJSON := `{
		"resourceLogs": [{
			"scopeLogs": [{
				"logRecords": [{
					"attributes": [
						{"key": "duplicateKey", "value": {"stringValue": "first"}},
						{"key": "normalKey", "value": {"stringValue": "normal"}},
						{"key": "duplicateKey", "value": {"stringValue": "second"}}
					]
				}]
			}]
		}]
	}`

	unmarshaler := &plog.JSONUnmarshaler{}
	logs, err := unmarshaler.UnmarshalLogs([]byte(otlpJSON))
	require.NoError(t, err)

	attributes := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()

	// Verify the parsed attributes contain duplicates (3 entries including duplicate)
	require.Equal(t, 3, attributes.Len())

	// Serialize using writeAttributes
	var buf bytes.Buffer
	visitor := structformjson.NewVisitor(&buf)
	require.NoError(t, visitor.OnObjectStart(-1, structform.AnyType))
	writeAttributes(visitor, attributes, false)
	require.NoError(t, visitor.OnObjectFinished())

	// Parse and verify the output
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&result))

	// Verify deduplication: only 2 keys, and duplicateKey has the first value
	expected := map[string]any{
		"attributes": map[string]any{
			"duplicateKey": "first",
			"normalKey":    "normal",
		},
	}
	assert.Equal(t, expected, result)
}
