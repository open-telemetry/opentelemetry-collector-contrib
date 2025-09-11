// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package avrologencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/avrologencodingextension"

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/linkedin/goavro/v2"
	"github.com/stretchr/testify/require"
)

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

func createAVROTestData(t *testing.T) (string, []byte) {
	t.Helper()

	schema, err := loadAVROSchemaFromFile("testdata/schema1.avro")
	require.NoError(t, err, "Failed to read avro schema file")

	codec, err := goavro.NewCodec(string(schema))
	require.NoError(t, err, "Failed to create avro code from schema")

	data := encodeAVROLogTestData(codec, `{
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

	return string(schema), data
}
