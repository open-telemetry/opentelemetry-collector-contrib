// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"

const (
	// SignalHeaderKey is the Kafka record header that identifies the telemetry signal.
	SignalHeaderKey = "otel.signal"

	SignalLogs     = "logs"
	SignalMetrics  = "metrics"
	SignalTraces   = "traces"
	SignalProfiles = "profiles"
)
