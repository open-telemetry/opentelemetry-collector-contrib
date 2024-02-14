// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/zipkinencodingextension"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv1"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv2"
)

const (
	zipkinProtobufEncoding = "zipkin_proto"
	zipkinJSONEncoding     = "zipkin_json"
	zipkinThriftEncoding   = "zipkin_thrift"
	v1                     = "v1"
	v2                     = "v2"
)

var (
	_ encoding.TracesMarshalerExtension   = (*zipkinExtension)(nil)
	_ encoding.TracesUnmarshalerExtension = (*zipkinExtension)(nil)
)

type zipkinExtension struct {
	config      *Config
	marshaler   ptrace.Marshaler
	unmarshaler ptrace.Unmarshaler
}

func newExtension(config *Config) (*zipkinExtension, error) {
	var err error
	var ex *zipkinExtension

	protocol := config.Protocol
	version := config.Version
	switch protocol {
	case zipkinProtobufEncoding:
		switch version {
		case v2:
			ex = &zipkinExtension{
				config:      config,
				marshaler:   zipkinv2.NewProtobufTracesMarshaler(),
				unmarshaler: zipkinv2.NewProtobufTracesUnmarshaler(false, false),
			}
		default:
			err = fmt.Errorf("protocol: %q, unsupported version: %q", protocol, version)
		}
	case zipkinJSONEncoding:
		switch version {
		case v1:
			ex = &zipkinExtension{
				config:      config,
				marshaler:   nil,
				unmarshaler: zipkinv1.NewJSONTracesUnmarshaler(false),
			}
		case v2:
			ex = &zipkinExtension{
				config:      config,
				marshaler:   zipkinv2.NewJSONTracesMarshaler(),
				unmarshaler: zipkinv2.NewJSONTracesUnmarshaler(false),
			}
		}
	case zipkinThriftEncoding:
		switch version {
		case v1:
			ex = &zipkinExtension{
				config:      config,
				marshaler:   nil,
				unmarshaler: zipkinv1.NewThriftTracesUnmarshaler(),
			}
		default:
			err = fmt.Errorf("protocol: %q, unsupported version: %q", protocol, version)
		}
	default:
		err = fmt.Errorf("unsupported protocol: %q", protocol)
	}

	return ex, err
}

func (ex *zipkinExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (ex *zipkinExtension) Shutdown(_ context.Context) error {
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
