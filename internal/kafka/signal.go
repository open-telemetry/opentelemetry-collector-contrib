// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"

// SignalHeaderKey is the Kafka record header that identifies the telemetry
// signal. Values are pipeline.Signal.String(): logs, metrics, traces, or
// profiles.
const SignalHeaderKey = "otel.signal"
