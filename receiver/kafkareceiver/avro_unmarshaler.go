// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"errors"
	"io"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	avroExt "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/avrologencodingextension"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	avroEncoding = "avro"
)

var (
	errFailedToReadAvroSchema      = errors.New("failed to read avro schema")
	errAvroEncodingExtensionNotSet = errors.New("avro encoding extension not set")
	errAvroEncodingExtension       = errors.New("failed to create avro encoding extension")
)

type avroLogsUnmarshaler struct {
	encodingExtension encoding.LogsUnmarshalerExtension
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

	cfg := &avroExt.Config{
		Schema: string(schema),
	}

	encodingExtension, err := avroExt.NewFactory().CreateExtension(
		context.Background(),
		extension.CreateSettings{},
		cfg)
	if err != nil {
		return nil, errors.Join(errAvroEncodingExtension, err)
	}

	a.encodingExtension = encodingExtension.(encoding.LogsUnmarshalerExtension)

	return a, err
}

func (a *avroLogsUnmarshaler) Unmarshal(data []byte) (plog.Logs, error) {
	if a.encodingExtension == nil {
		return plog.NewLogs(), errAvroEncodingExtensionNotSet
	}

	return a.encodingExtension.UnmarshalLogs(data)
}
