// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
    "go.opentelemetry.io/collector/pdata/plog"
    "go.opentelemetry.io/collector/pdata/pmetric"
    "go.opentelemetry.io/collector/pdata/ptrace"
)

const (
    otlpJSONEncoding = "otlp_json"
)

// newOTLPJSONTracesUnmarshaler creates a new OTLP JSON traces unmarshaler.
func newOTLPJSONTracesUnmarshaler() TracesUnmarshaler {
    return newPdataTracesUnmarshaler(&ptrace.JSONUnmarshaler{}, otlpJSONEncoding)
}

// newOTLPJSONMetricsUnmarshaler creates a new OTLP JSON metrics unmarshaler.
func newOTLPJSONMetricsUnmarshaler() MetricsUnmarshaler {
    return newPdataMetricsUnmarshaler(&pmetric.JSONUnmarshaler{}, otlpJSONEncoding)
}

// newOTLPJSONLogsUnmarshaler creates a new OTLP JSON logs unmarshaler.
func newOTLPJSONLogsUnmarshaler() LogsUnmarshaler {
    return newPdataLogsUnmarshaler(&plog.JSONUnmarshaler{}, otlpJSONEncoding)
}