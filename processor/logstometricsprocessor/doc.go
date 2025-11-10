// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package logstometricsprocessor extracts metrics from logs using OTTL expressions.
// The processor supports extracting Sum, Gauge, Histogram, and Exponential Histogram
// metrics from log records and can optionally forward or drop logs after processing.
package logstometricsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor"

