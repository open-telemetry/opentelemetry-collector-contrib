// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/zipkinencodingextension"

import (
	"context"
	"errors"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv1"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	zipkinProtobufEncoding = "zipkin_proto"
	zipkinJSONEncoding     = "zipkin_json"
	zipkinThriftEncoding   = "zipkin_thrift"
	v1                     = "v1"
	v2                     = "v2"
)

var _ ptrace.Marshaler = &zipkinExtension{}
var _ ptrace.Unmarshaler = &zipkinExtension{}

type zipkinExtension struct {
	config      *Config
	marshaler   ptrace.Marshaler
	unmarshaler ptrace.Unmarshaler
}

func (ex *zipkinExtension) Start(ctx context.Context, host component.Host) error {
	protocol := ex.config.Protocol
	version := ex.config.Version

	switch protocol {
	case zipkinProtobufEncoding:
		switch version {
		case v1:
			return fmt.Errorf("unsupported version: %q and protocol: %q", version, protocol)
		case v2:
			ex.marshaler = zipkinv2.NewProtobufTracesMarshaler()
			ex.unmarshaler = zipkinv2.NewProtobufTracesUnmarshaler(false, false)
		default:
			return fmt.Errorf("unsupported version: %q and protocol: %q", version, protocol)
		}
	case zipkinJSONEncoding:
		switch version {
		case v1:
			ex.marshaler = nil
			ex.unmarshaler = zipkinv1.NewJSONTracesUnmarshaler(false)
		case v2:
			ex.marshaler = zipkinv2.NewJSONTracesMarshaler()
			ex.unmarshaler = zipkinv2.NewJSONTracesUnmarshaler(false)
		default:
			return fmt.Errorf("unsupported version: %q and protocol: %q", version, protocol)
		}
	case zipkinThriftEncoding:
		switch version {
		case v1:
			ex.marshaler = nil
			ex.unmarshaler = zipkinv1.NewThriftTracesUnmarshaler()
		default:
			return fmt.Errorf("unsupported version: %q and protocol: %q", version, protocol)
		}
	default:
		return fmt.Errorf("unsupported version: %q and protocol: %q", version, protocol)
	}

	return nil
}

func (ex *zipkinExtension) Shutdown(ctx context.Context) error {
	return nil
}

func (ex *zipkinExtension) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	if ex.unmarshaler == nil {
		return ptrace.Traces{}, errors.New("unsupported encoding")
	}
	return ex.unmarshaler.UnmarshalTraces(buf)
}

func (ex *zipkinExtension) MarshalTraces(td ptrace.Traces) ([]byte, error) {
	if ex.unmarshaler == nil {
		return nil, errors.New("unsupported encoding")
	}
	return ex.marshaler.MarshalTraces(td)
}
