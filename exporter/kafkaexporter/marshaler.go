// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv2"
)

var errUnknownEncodingExtension = errors.New("unknown encoding extension")

func getTracesMarshaler(encoding string, host component.Host) (marshaler.TracesMarshaler, error) {
	if m, err := loadEncodingExtension[ptrace.Marshaler](host, encoding, "traces"); err != nil {
		if !errors.Is(err, errUnknownEncodingExtension) {
			return nil, err
		}
	} else {
		return marshaler.NewPdataTracesMarshaler(m), nil
	}
	switch encoding {
	case "otlp_proto":
		return marshaler.NewPdataTracesMarshaler(&ptrace.ProtoMarshaler{}), nil
	case "otlp_json":
		return marshaler.NewPdataTracesMarshaler(&ptrace.JSONMarshaler{}), nil
	case "zipkin_proto":
		return marshaler.NewPdataTracesMarshaler(zipkinv2.NewProtobufTracesMarshaler()), nil
	case "zipkin_json":
		return marshaler.NewPdataTracesMarshaler(zipkinv2.NewJSONTracesMarshaler()), nil
	case "jaeger_proto":
		return marshaler.JaegerProtoSpanMarshaler{}, nil
	case "jaeger_json":
		return marshaler.JaegerJSONSpanMarshaler{}, nil
	}
	return nil, fmt.Errorf("unrecognized traces encoding %q", encoding)
}

func getMetricsMarshaler(encoding string, host component.Host) (marshaler.MetricsMarshaler, error) {
	if m, err := loadEncodingExtension[pmetric.Marshaler](host, encoding, "metrics"); err != nil {
		if !errors.Is(err, errUnknownEncodingExtension) {
			return nil, err
		}
	} else {
		return marshaler.NewPdataMetricsMarshaler(m), nil
	}
	switch encoding {
	case "otlp_proto":
		return marshaler.NewPdataMetricsMarshaler(&pmetric.ProtoMarshaler{}), nil
	case "otlp_json":
		return marshaler.NewPdataMetricsMarshaler(&pmetric.JSONMarshaler{}), nil
	}
	return nil, fmt.Errorf("unrecognized metrics encoding %q", encoding)
}

func getLogsMarshaler(encoding string, host component.Host) (marshaler.LogsMarshaler, error) {
	if m, err := loadEncodingExtension[plog.Marshaler](host, encoding, "logs"); err != nil {
		if !errors.Is(err, errUnknownEncodingExtension) {
			return nil, err
		}
	} else {
		return marshaler.NewPdataLogsMarshaler(m), nil
	}
	switch encoding {
	case "otlp_proto":
		return marshaler.NewPdataLogsMarshaler(&plog.ProtoMarshaler{}), nil
	case "otlp_json":
		return marshaler.NewPdataLogsMarshaler(&plog.JSONMarshaler{}), nil
	case "raw":
		return marshaler.RawLogsMarshaler{}, nil
	}
	return nil, fmt.Errorf("unrecognized logs encoding %q", encoding)
}

// loadEncodingExtension tries to load an available extension for the given encoding.
func loadEncodingExtension[T any](host component.Host, encoding, signalType string) (T, error) {
	var zero T
	extensionID, err := encodingToComponentID(encoding)
	if err != nil {
		return zero, err
	}
	encodingExtension, ok := host.GetExtensions()[*extensionID]
	if !ok {
		return zero, fmt.Errorf("invalid encoding %q: %w", encoding, errUnknownEncodingExtension)
	}
	marshaler, ok := encodingExtension.(T)
	if !ok {
		return zero, fmt.Errorf("extension %q is not a %s marshaler", encoding, signalType)
	}
	return marshaler, nil
}

// encodingToComponentID attempts to parse the encoding string as a component ID.
func encodingToComponentID(encoding string) (*component.ID, error) {
	var id component.ID
	if err := id.UnmarshalText([]byte(encoding)); err != nil {
		return nil, fmt.Errorf("invalid component ID: %w", err)
	}
	return &id, nil
}
