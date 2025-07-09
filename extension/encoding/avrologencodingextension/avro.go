// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package avrologencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/avrologencodingextension"

import (
	"fmt"

	"github.com/linkedin/goavro/v2"
)

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

	return native.(map[string]any), nil
}
