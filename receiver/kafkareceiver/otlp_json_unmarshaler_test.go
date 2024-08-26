// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "go.opentelemetry.io/collector/pdata/ptrace"
)

func TestNewOTLPJSONTracesUnmarshaler(t *testing.T) {
    t.Parallel()
    um := newOTLPJSONTracesUnmarshaler()
    assert.Equal(t, otlpJSONEncoding, um.Encoding())
}

func TestOTLPJSONTracesUnmarshalerReturnType(t *testing.T) {
    t.Parallel()
    um := newOTLPJSONTracesUnmarshaler()
    json := `{"resourceSpans":[{"resource":{},"scopeSpans":[{"scope":{},"spans":[]}]}]}`
    unmarshaledTraces, err := um.Unmarshal([]byte(json))
    assert.NoError(t, err)
    var expectedType ptrace.Traces
    assert.IsType(t, expectedType, unmarshaledTraces)
}