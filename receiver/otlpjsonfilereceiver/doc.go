// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package otlpjsonfilereceiver implements a receiver that can be used by the
// Opentelemetry collector to receive logs, traces and metrics from files
// See https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/file-exporter.md#json-file-serialization
package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"
