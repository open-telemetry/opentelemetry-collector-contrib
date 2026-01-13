// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type marshaler interface {
	MarshalLogs(ld plog.Logs) ([]byte, error)
	format() string
	compressed() bool
}

var ErrUnknownMarshaler = errors.New("unknown marshaler")

func newMarshalerFromEncoding(encoding *component.ID, fileFormat string, host component.Host, logger *zap.Logger) (marshaler, error) {
	marshaler := &gcsMarshaler{logger: logger}
	e, ok := host.GetExtensions()[*encoding]
	if !ok {
		return nil, fmt.Errorf("unknown extension %q", encoding)
	}
	// Check if the extension implements the marshaler interface
	logsMarshaler, ok := e.(plog.Marshaler)
	if !ok {
		return nil, fmt.Errorf("extension %q is not a logs marshaler", encoding)
	}
	marshaler.logsMarshaler = logsMarshaler
	marshaler.fileFormat = fileFormat
	marshaler.IsCompressed = false
	return marshaler, nil
}

func newMarshaler(mType string, logger *zap.Logger) (marshaler, error) {
	marshaler := &gcsMarshaler{logger: logger}
	switch mType {
	case "json":
		marshaler.logsMarshaler = &plog.JSONMarshaler{}
		marshaler.fileFormat = "json"
		marshaler.IsCompressed = false
	default:
		return nil, ErrUnknownMarshaler
	}
	return marshaler, nil
}
