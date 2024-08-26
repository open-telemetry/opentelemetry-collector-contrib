// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
    "go.opentelemetry.io/collector/pdata/ptrace"
)

const (
    otlpJSONEncoding = "otlp_json"
)

// otlpJSONTracesUnmarshaler unmarshals OTLP JSON-encoded traces.
type otlpJSONTracesUnmarshaler struct {
    ptrace.Unmarshaler
}

func (o otlpJSONTracesUnmarshaler) Unmarshal(buf []byte) (ptrace.Traces, error) {
    return o.Unmarshaler.UnmarshalTraces(buf)
}

func (o otlpJSONTracesUnmarshaler) Encoding() string {
    return otlpJSONEncoding
}

// newOTLPJSONTracesUnmarshaler creates a new OTLP JSON traces unmarshaler.
func newOTLPJSONTracesUnmarshaler() TracesUnmarshaler {
    return &otlpJSONTracesUnmarshaler{
        Unmarshaler: &ptrace.JSONUnmarshaler{},
    }
}