// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// errUnknownEncodingExtension indicates the encoding did not resolve to a
// registered encoding extension, so a built-in encoding should be tried.
var errUnknownEncodingExtension = errors.New("unknown encoding extension")

func getTracesMarshaler(encoding string, host component.Host) (ptrace.Marshaler, error) {
	if m, err := loadEncodingExtension[ptrace.Marshaler](host, encoding); err != nil {
		if !errors.Is(err, errUnknownEncodingExtension) {
			return nil, err
		}
	} else {
		return m, nil
	}
	switch encoding {
	case "otlp_proto":
		return &ptrace.ProtoMarshaler{}, nil
	case "otlp_json":
		return &ptrace.JSONMarshaler{}, nil
	}
	return nil, fmt.Errorf("unrecognized traces encoding %q", encoding)
}

func getMetricsMarshaler(encoding string, host component.Host) (pmetric.Marshaler, error) {
	if m, err := loadEncodingExtension[pmetric.Marshaler](host, encoding); err != nil {
		if !errors.Is(err, errUnknownEncodingExtension) {
			return nil, err
		}
	} else {
		return m, nil
	}
	switch encoding {
	case "otlp_proto":
		return &pmetric.ProtoMarshaler{}, nil
	case "otlp_json":
		return &pmetric.JSONMarshaler{}, nil
	}
	return nil, fmt.Errorf("unrecognized metrics encoding %q", encoding)
}

func getLogsMarshaler(encoding string, host component.Host) (plog.Marshaler, error) {
	if m, err := loadEncodingExtension[plog.Marshaler](host, encoding); err != nil {
		if !errors.Is(err, errUnknownEncodingExtension) {
			return nil, err
		}
	} else {
		return m, nil
	}
	switch encoding {
	case "otlp_proto":
		return &plog.ProtoMarshaler{}, nil
	case "otlp_json":
		return &plog.JSONMarshaler{}, nil
	}
	return nil, fmt.Errorf("unrecognized logs encoding %q", encoding)
}

// loadEncodingExtension resolves the encoding string to a registered encoding
// extension implementing the requested marshaler type T. It returns
// errUnknownEncodingExtension when no such extension is registered, so callers
// can fall back to the built-in encodings.
func loadEncodingExtension[T any](host component.Host, encoding string) (T, error) {
	var zero T
	id, err := encodingToComponentID(encoding)
	if err != nil {
		return zero, err
	}
	ext, ok := host.GetExtensions()[*id]
	if !ok {
		return zero, fmt.Errorf("invalid encoding %q: %w", encoding, errUnknownEncodingExtension)
	}
	marshaler, ok := ext.(T)
	if !ok {
		return zero, fmt.Errorf("extension %q is not a marshaler for this signal type", encoding)
	}
	return marshaler, nil
}

// encodingToComponentID converts an encoding string to a component.ID, returning
// an error if the encoding string is not a valid component ID.
func encodingToComponentID(encoding string) (*component.ID, error) {
	var id component.ID
	if err := id.UnmarshalText([]byte(encoding)); err != nil {
		return nil, fmt.Errorf("invalid component ID %q: %w", encoding, err)
	}
	return &id, nil
}
