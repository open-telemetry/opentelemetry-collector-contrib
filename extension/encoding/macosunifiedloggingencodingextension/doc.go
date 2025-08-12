// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package macosunifiedloggingencodingextension provides an encoding extension
// for decoding macOS Unified Logging binary files (tracev3 format).
//
// This extension reads binary data from macOS Unified Logging archives and
// converts them into OpenTelemetry log records. It is designed to work with
// the macOS Unified Logging receiver to parse system logs on macOS platforms.
//
// The extension implements the LogsUnmarshalerExtension interface to decode
// binary log data into structured log records that can be processed by the
// OpenTelemetry Collector.
package macosunifiedloggingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension"
